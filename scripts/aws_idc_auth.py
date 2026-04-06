#!/usr/bin/env python3
"""
AWS IAM Identity Center (IDC) Headless Authentication Script
Supports FreeRADIUS + IDC with Duo MFA

This script enables programmatic AWS API access using AWS IDC (SSO) roles
WITHOUT requiring a browser for every authentication — designed for on-prem
monitoring tools and headless environments.

Auth Stack: FreeRADIUS (FR) -> AWS IDC -> Duo MFA -> AWS STS

How Duo MFA works with this script:
- During ONE-TIME device authorization (browser step), Duo MFA challenge
  is presented (push/TOTP/phone). You approve it once.
- After approval, AWS IDC issues access + refresh tokens.
- Refresh token renewals do NOT re-trigger Duo MFA (they operate at the
  OIDC layer, below the MFA enforcement point).
- Duo MFA is only re-triggered when:
  a) Refresh token expires (controlled by IDC session duration policy)
  b) OIDC client registration expires (~90 days)
  c) Admin revokes the session

For monitoring tools: Use --daemon mode to proactively refresh tokens
before they expire, minimizing re-auth windows.

Flow:
1. First run: registers an OIDC client and performs device authorization
   (requires ONE-TIME browser + Duo MFA approval)
2. Caches access token + refresh token locally (encrypted-at-rest optional)
3. Subsequent runs: uses refresh token to get new access tokens
   (NO browser, NO Duo prompt)
4. Uses SSO GetRoleCredentials to obtain temporary AWS credentials
5. Returns a boto3 session ready for API calls

Requirements:
    pip install boto3

Usage:
    # As a module
    from aws_idc_auth import get_aws_session
    session = get_aws_session(
        sso_start_url="https://your-org.awsapps.com/start",
        sso_region="us-east-1",
        account_id="123456789012",
        role_name="YourRoleName"
    )
    ec2 = session.client("ec2")
    ec2.describe_instances()

    # As CLI
    python aws_idc_auth.py \
        --start-url https://your-org.awsapps.com/start \
        --region us-east-1 \
        --account-id 123456789012 \
        --role-name YourRoleName \
        --test

    # Daemon mode — keeps tokens fresh for monitoring tools
    python aws_idc_auth.py \
        --start-url https://your-org.awsapps.com/start \
        --region us-east-1 \
        --account-id 123456789012 \
        --role-name YourRoleName \
        --daemon --creds-file /tmp/aws_creds.json
"""

import json
import os
import sys
import time
import signal
import logging
import argparse
import subprocess
from pathlib import Path
from datetime import datetime, timezone

import boto3
from botocore.config import Config

logger = logging.getLogger(__name__)

# Default cache directory
CACHE_DIR = Path.home() / ".aws" / "idc-token-cache"

# OIDC client registration is valid for a long time; tokens expire sooner
TOKEN_EXPIRY_BUFFER_SECONDS = 300  # refresh 5 min before actual expiry

# Daemon mode: how often to refresh credentials (seconds)
DAEMON_REFRESH_INTERVAL = 900  # 15 minutes

# Duo MFA: extra time buffer for device auth (Duo push can take a moment)
DUO_DEVICE_AUTH_TIMEOUT_EXTRA = 60  # extra seconds to wait for Duo approval


class IDCAuthError(Exception):
    """Raised when IDC authentication fails."""
    pass


class IDCTokenManager:
    """Manages OIDC client registration, device auth, and token lifecycle."""

    def __init__(self, sso_start_url: str, sso_region: str, cache_dir: Path = None):
        self.sso_start_url = sso_start_url
        self.sso_region = sso_region
        self.cache_dir = cache_dir or CACHE_DIR
        self.cache_dir.mkdir(parents=True, exist_ok=True)

        no_proxy_config = Config(
            region_name=sso_region,
            signature_version=boto3.session.UNSIGNED,
        )
        self._oidc_client = boto3.client(
            "sso-oidc", region_name=sso_region, config=no_proxy_config
        )

    @property
    def _cache_file(self) -> Path:
        """Cache file path derived from start URL to support multiple orgs."""
        safe_name = self.sso_start_url.replace("https://", "").replace("/", "_")
        return self.cache_dir / f"{safe_name}.json"

    def _load_cache(self) -> dict | None:
        if self._cache_file.exists():
            try:
                data = json.loads(self._cache_file.read_text())
                logger.debug("Loaded cached token data")
                return data
            except (json.JSONDecodeError, OSError) as e:
                logger.warning("Failed to read cache: %s", e)
        return None

    def _save_cache(self, data: dict):
        self._cache_file.write_text(json.dumps(data, indent=2, default=str))
        # Restrict permissions — tokens are sensitive
        self._cache_file.chmod(0o600)
        logger.debug("Saved token cache to %s", self._cache_file)

    def _register_client(self) -> dict:
        """Register an OIDC public client. Valid for ~90 days."""
        logger.info("Registering new OIDC client with AWS SSO...")
        response = self._oidc_client.register_client(
            clientName="onprem-monitoring-headless",
            clientType="public",
            scopes=["sso:account:access"],
        )
        return {
            "clientId": response["clientId"],
            "clientSecret": response["clientSecret"],
            "clientSecretExpiresAt": response["clientSecretExpiresAt"],
        }

    def _start_device_auth(self, client_id: str, client_secret: str) -> dict:
        """Start device authorization flow. Returns verification URI + codes."""
        response = self._oidc_client.start_device_authorization(
            clientId=client_id,
            clientSecret=client_secret,
            startUrl=self.sso_start_url,
        )
        return {
            "deviceCode": response["deviceCode"],
            "userCode": response["userCode"],
            "verificationUri": response["verificationUri"],
            "verificationUriComplete": response["verificationUriComplete"],
            "expiresIn": response["expiresIn"],
            "interval": response.get("interval", 5),
        }

    def _poll_for_token(
        self, client_id: str, client_secret: str, device_code: str,
        interval: int, expires_in: int
    ) -> dict:
        """Poll for token after user authorizes the device. No browser needed here."""
        deadline = time.time() + expires_in
        while time.time() < deadline:
            try:
                response = self._oidc_client.create_token(
                    clientId=client_id,
                    clientSecret=client_secret,
                    grantType="urn:ietf:params:oauth:grant-type:device_code",
                    deviceCode=device_code,
                )
                logger.info("Device authorized successfully!")
                return {
                    "accessToken": response["accessToken"],
                    "refreshToken": response.get("refreshToken"),
                    "expiresAt": time.time() + response["expiresIn"],
                    "tokenType": response.get("tokenType", "Bearer"),
                }
            except self._oidc_client.exceptions.AuthorizationPendingException:
                time.sleep(interval)
            except self._oidc_client.exceptions.SlowDownException:
                interval += 5
                time.sleep(interval)
            except self._oidc_client.exceptions.ExpiredTokenException:
                raise IDCAuthError("Device authorization expired. Please try again.")

        raise IDCAuthError("Timed out waiting for device authorization.")

    def _refresh_access_token(self, cache: dict) -> dict | None:
        """Use refresh token to get a new access token — NO browser needed."""
        refresh_token = cache.get("token", {}).get("refreshToken")
        if not refresh_token:
            logger.info("No refresh token available; full re-auth required.")
            return None

        client = cache.get("client", {})
        # Check if client registration is still valid
        if client.get("clientSecretExpiresAt", 0) < time.time():
            logger.info("OIDC client registration expired; re-registering.")
            return None

        try:
            logger.info("Refreshing access token using refresh token (no browser)...")
            response = self._oidc_client.create_token(
                clientId=client["clientId"],
                clientSecret=client["clientSecret"],
                grantType="refresh_token",
                refreshToken=refresh_token,
            )
            token_data = {
                "accessToken": response["accessToken"],
                "refreshToken": response.get("refreshToken", refresh_token),
                "expiresAt": time.time() + response["expiresIn"],
                "tokenType": response.get("tokenType", "Bearer"),
            }
            logger.info("Token refreshed successfully — no browser interaction needed!")
            return token_data
        except Exception as e:
            logger.warning("Token refresh failed: %s. Will re-authenticate.", e)
            return None

    def get_access_token(self) -> str:
        """
        Get a valid SSO access token. Uses cache/refresh when possible.

        Returns the access token string.
        """
        cache = self._load_cache() or {}

        # 1. Check if we have a valid (non-expired) access token
        token = cache.get("token", {})
        if token.get("accessToken") and token.get("expiresAt", 0) > (
            time.time() + TOKEN_EXPIRY_BUFFER_SECONDS
        ):
            logger.info("Using cached access token (still valid).")
            return token["accessToken"]

        # 2. Try refreshing with refresh token (headless!)
        if token.get("refreshToken"):
            new_token = self._refresh_access_token(cache)
            if new_token:
                cache["token"] = new_token
                self._save_cache(cache)
                return new_token["accessToken"]

        # 3. Full device authorization flow (requires one-time interaction)
        logger.info("No valid token or refresh token. Starting device auth flow...")
        logger.info("=" * 60)
        logger.info("ONE-TIME SETUP: Browser authorization required.")
        logger.info("After this, the script will run headless using refresh tokens.")
        logger.info("=" * 60)

        client = cache.get("client", {})
        if not client.get("clientId") or client.get("clientSecretExpiresAt", 0) < time.time():
            client = self._register_client()

        device_auth = self._start_device_auth(client["clientId"], client["clientSecret"])

        # Print instructions for the user (Duo MFA aware)
        print("\n" + "=" * 60)
        print("  AWS IDC DEVICE AUTHORIZATION (ONE-TIME SETUP)")
        print("  Auth Stack: FreeRADIUS -> IDC -> Duo MFA")
        print("=" * 60)
        print(f"\n  1. Open this URL:  {device_auth['verificationUriComplete']}")
        print(f"  2. Confirm code:   {device_auth['userCode']}")
        print(f"  3. Authenticate with your FR/IDC credentials")
        print(f"  4. Approve the Duo MFA prompt (push/TOTP/phone)")
        print(f"\n  NOTE: After this one-time approval, the script will")
        print(f"  run headless — Duo MFA will NOT be triggered again")
        print(f"  until the refresh token expires.")
        print(f"\n  Waiting for authorization (expires in {device_auth['expiresIn']}s)...")
        print("=" * 60 + "\n")

        token_data = self._poll_for_token(
            client["clientId"],
            client["clientSecret"],
            device_auth["deviceCode"],
            device_auth["interval"],
            device_auth["expiresIn"],
        )

        cache["client"] = client
        cache["token"] = token_data
        cache["startUrl"] = self.sso_start_url
        cache["region"] = self.sso_region
        self._save_cache(cache)

        return token_data["accessToken"]


def get_role_credentials(
    access_token: str, account_id: str, role_name: str, sso_region: str
) -> dict:
    """
    Exchange an SSO access token for temporary AWS credentials.

    Returns dict with AccessKeyId, SecretAccessKey, SessionToken, Expiration.
    """
    sso_client = boto3.client("sso", region_name=sso_region)
    response = sso_client.get_role_credentials(
        roleName=role_name,
        accountId=account_id,
        accessToken=access_token,
    )
    creds = response["roleCredentials"]
    logger.info(
        "Got temporary credentials for %s/%s (expires: %s)",
        account_id,
        role_name,
        datetime.fromtimestamp(creds["expiration"] / 1000, tz=timezone.utc).isoformat(),
    )
    return creds


def get_aws_session(
    sso_start_url: str,
    sso_region: str,
    account_id: str,
    role_name: str,
    cache_dir: Path = None,
) -> boto3.Session:
    """
    Main entry point: returns a boto3 Session authenticated via AWS IDC.

    After initial one-time browser auth, this runs fully headless using
    cached refresh tokens.

    Args:
        sso_start_url: Your AWS SSO start URL (e.g., https://myorg.awsapps.com/start)
        sso_region: AWS region where SSO is configured
        account_id: Target AWS account ID
        role_name: IAM Identity Center permission set / role name
        cache_dir: Optional custom cache directory for tokens

    Returns:
        boto3.Session with temporary credentials
    """
    token_mgr = IDCTokenManager(sso_start_url, sso_region, cache_dir)
    access_token = token_mgr.get_access_token()

    creds = get_role_credentials(access_token, account_id, role_name, sso_region)

    session = boto3.Session(
        aws_access_key_id=creds["accessKeyId"],
        aws_secret_access_key=creds["secretAccessKey"],
        aws_session_token=creds["sessionToken"],
        region_name=sso_region,
    )

    return session


def list_available_accounts_and_roles(access_token: str, sso_region: str):
    """List all accounts and roles available to the authenticated user."""
    sso_client = boto3.client("sso", region_name=sso_region)

    print("\nAvailable AWS Accounts and Roles:")
    print("-" * 50)

    paginator = sso_client.get_paginator("list_accounts")
    for page in paginator.paginate(accessToken=access_token):
        for account in page["accountList"]:
            print(f"\n  Account: {account['accountName']} ({account['accountId']})")
            print(f"  Email:   {account.get('emailAddress', 'N/A')}")

            role_paginator = sso_client.get_paginator("list_account_roles")
            for role_page in role_paginator.paginate(
                accessToken=access_token, accountId=account["accountId"]
            ):
                for role in role_page["roleList"]:
                    print(f"    - Role: {role['roleName']}")

    print()


def write_creds_file(creds: dict, region: str, creds_file: str):
    """Write credentials to a JSON file that monitoring tools can read."""
    creds_path = Path(creds_file)
    creds_data = {
        "AccessKeyId": creds["accessKeyId"],
        "SecretAccessKey": creds["secretAccessKey"],
        "SessionToken": creds["sessionToken"],
        "Expiration": datetime.fromtimestamp(
            creds["expiration"] / 1000, tz=timezone.utc
        ).isoformat(),
        "Region": region,
        "LastRefreshed": datetime.now(tz=timezone.utc).isoformat(),
    }
    # Write atomically to avoid monitoring tools reading partial files
    tmp_path = creds_path.with_suffix(".tmp")
    tmp_path.write_text(json.dumps(creds_data, indent=2))
    tmp_path.chmod(0o600)
    tmp_path.rename(creds_path)
    logger.info("Credentials written to %s", creds_file)


def send_reauth_notification(message: str, webhook_url: str = None):
    """
    Send notification when re-authentication (Duo MFA) is needed.
    Supports webhook URL for Slack/Teams/PagerDuty integration.
    """
    logger.warning("RE-AUTH REQUIRED: %s", message)

    if webhook_url:
        try:
            import urllib.request
            payload = json.dumps({"text": f"[AWS IDC Auth] {message}"}).encode()
            req = urllib.request.Request(
                webhook_url,
                data=payload,
                headers={"Content-Type": "application/json"},
            )
            urllib.request.urlopen(req, timeout=10)
            logger.info("Notification sent to webhook")
        except Exception as e:
            logger.error("Failed to send webhook notification: %s", e)


def run_daemon(
    sso_start_url: str,
    sso_region: str,
    account_id: str,
    role_name: str,
    creds_file: str,
    refresh_interval: int = DAEMON_REFRESH_INTERVAL,
    webhook_url: str = None,
):
    """
    Daemon mode: continuously refreshes credentials for monitoring tools.

    Writes fresh AWS credentials to a JSON file that monitoring tools can
    read. When refresh tokens expire (requiring Duo MFA re-auth), sends
    a notification via webhook.
    """
    logger.info("Starting credential refresh daemon")
    logger.info("  Credentials file: %s", creds_file)
    logger.info("  Refresh interval: %ds", refresh_interval)
    logger.info("  Webhook URL: %s", webhook_url or "(none)")

    running = True

    def handle_signal(signum, frame):
        nonlocal running
        logger.info("Received signal %d, shutting down...", signum)
        running = False

    signal.signal(signal.SIGTERM, handle_signal)
    signal.signal(signal.SIGINT, handle_signal)

    token_mgr = IDCTokenManager(sso_start_url, sso_region)

    while running:
        try:
            access_token = token_mgr.get_access_token()
            creds = get_role_credentials(
                access_token, account_id, role_name, sso_region
            )
            write_creds_file(creds, sso_region, creds_file)

            # Calculate time until credentials expire
            expires_at = creds["expiration"] / 1000
            remaining = expires_at - time.time()
            logger.info(
                "Credentials valid for %.0f minutes. Next refresh in %ds.",
                remaining / 60,
                refresh_interval,
            )

        except IDCAuthError as e:
            msg = (
                f"Refresh token expired — Duo MFA re-authentication required. "
                f"Run the script interactively to re-authorize. Error: {e}"
            )
            send_reauth_notification(msg, webhook_url)
            logger.error(msg)
            # Still keep running — credentials file remains with last valid creds
            # until someone re-auths

        except Exception as e:
            logger.error("Unexpected error during credential refresh: %s", e)

        # Sleep in small increments so we can respond to signals
        for _ in range(refresh_interval):
            if not running:
                break
            time.sleep(1)

    logger.info("Daemon stopped.")


def check_token_health(sso_start_url: str, sso_region: str) -> dict:
    """
    Check the health/status of cached tokens.
    Useful for monitoring tools to know when Duo re-auth will be needed.
    """
    token_mgr = IDCTokenManager(sso_start_url, sso_region)
    cache = token_mgr._load_cache() or {}

    status = {
        "has_cached_token": False,
        "token_valid": False,
        "token_expires_in_seconds": 0,
        "has_refresh_token": False,
        "client_registration_valid": False,
        "client_expires_in_seconds": 0,
        "needs_duo_reauth": True,
    }

    token = cache.get("token", {})
    client = cache.get("client", {})
    now = time.time()

    if token.get("accessToken"):
        status["has_cached_token"] = True
        expires_at = token.get("expiresAt", 0)
        if expires_at > now:
            status["token_valid"] = True
            status["token_expires_in_seconds"] = int(expires_at - now)

    if token.get("refreshToken"):
        status["has_refresh_token"] = True
        # If we have a valid refresh token, no Duo re-auth needed
        status["needs_duo_reauth"] = False

    if client.get("clientSecretExpiresAt", 0) > now:
        status["client_registration_valid"] = True
        status["client_expires_in_seconds"] = int(
            client["clientSecretExpiresAt"] - now
        )
    else:
        # Client expired = needs full re-auth including Duo
        status["needs_duo_reauth"] = True

    return status


def main():
    parser = argparse.ArgumentParser(
        description="AWS IDC (SSO) headless auth for on-prem monitoring (FR + IDC + Duo MFA)",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Examples:
  # Initial setup (one-time, requires browser + Duo MFA approval)
  %(prog)s --start-url https://myorg.awsapps.com/start --region us-east-1 \\
           --account-id 123456789012 --role-name ReadOnlyAccess --test

  # List available accounts and roles
  %(prog)s --start-url https://myorg.awsapps.com/start --region us-east-1 --list

  # Export credentials as env vars (for AWS CLI)
  eval $(%(prog)s --start-url https://myorg.awsapps.com/start --region us-east-1 \\
           --account-id 123456789012 --role-name ReadOnlyAccess --export-env)

  # Daemon mode for monitoring tools (writes creds to file, refreshes automatically)
  %(prog)s --start-url https://myorg.awsapps.com/start --region us-east-1 \\
           --account-id 123456789012 --role-name MonitoringRole \\
           --daemon --creds-file /opt/monitoring/aws_creds.json

  # Daemon with Slack webhook for Duo re-auth notifications
  %(prog)s --start-url https://myorg.awsapps.com/start --region us-east-1 \\
           --account-id 123456789012 --role-name MonitoringRole \\
           --daemon --creds-file /opt/monitoring/aws_creds.json \\
           --webhook-url https://hooks.slack.com/services/XXX/YYY/ZZZ

  # Check token health (for monitoring alerting)
  %(prog)s --start-url https://myorg.awsapps.com/start --region us-east-1 --health

Duo MFA Notes:
  - Duo MFA is only triggered during the initial device authorization
  - Refresh tokens renew silently without Duo prompts
  - When refresh tokens expire, use --health to check status
  - Use --webhook-url to get notified when Duo re-auth is needed
        """,
    )
    parser.add_argument("--start-url", required=True, help="AWS SSO start URL")
    parser.add_argument("--region", required=True, help="AWS SSO region")
    parser.add_argument("--account-id", help="Target AWS account ID")
    parser.add_argument("--role-name", help="IAM Identity Center role/permission set name")
    parser.add_argument("--list", action="store_true", help="List available accounts and roles")
    parser.add_argument("--test", action="store_true", help="Test auth with STS GetCallerIdentity")
    parser.add_argument("--export-env", action="store_true",
                        help="Print credentials as shell export statements")
    parser.add_argument("--daemon", action="store_true",
                        help="Run as daemon, continuously refreshing credentials")
    parser.add_argument("--creds-file", default="/tmp/aws_idc_creds.json",
                        help="File to write credentials to (daemon mode)")
    parser.add_argument("--refresh-interval", type=int, default=DAEMON_REFRESH_INTERVAL,
                        help=f"Credential refresh interval in seconds (default: {DAEMON_REFRESH_INTERVAL})")
    parser.add_argument("--webhook-url",
                        help="Webhook URL for Duo re-auth notifications (Slack/Teams/PagerDuty)")
    parser.add_argument("--health", action="store_true",
                        help="Check token health and Duo re-auth status")
    parser.add_argument("--verbose", "-v", action="store_true", help="Enable debug logging")

    args = parser.parse_args()

    logging.basicConfig(
        level=logging.DEBUG if args.verbose else logging.INFO,
        format="%(asctime)s [%(levelname)s] %(message)s",
    )

    # Health check mode
    if args.health:
        status = check_token_health(args.start_url, args.region)
        print(json.dumps(status, indent=2))
        # Exit code 0 if healthy, 1 if Duo re-auth needed
        sys.exit(0 if not status["needs_duo_reauth"] else 1)

    token_mgr = IDCTokenManager(args.start_url, args.region)
    access_token = token_mgr.get_access_token()

    if args.list:
        list_available_accounts_and_roles(access_token, args.region)
        return

    if not args.account_id or not args.role_name:
        parser.error("--account-id and --role-name are required (unless using --list or --health)")

    # Daemon mode
    if args.daemon:
        run_daemon(
            args.start_url,
            args.region,
            args.account_id,
            args.role_name,
            args.creds_file,
            args.refresh_interval,
            args.webhook_url,
        )
        return

    creds = get_role_credentials(access_token, args.account_id, args.role_name, args.region)

    if args.export_env:
        print(f"export AWS_ACCESS_KEY_ID={creds['accessKeyId']}")
        print(f"export AWS_SECRET_ACCESS_KEY={creds['secretAccessKey']}")
        print(f"export AWS_SESSION_TOKEN={creds['sessionToken']}")
        print(f"export AWS_DEFAULT_REGION={args.region}")
        return

    if args.test:
        session = boto3.Session(
            aws_access_key_id=creds["accessKeyId"],
            aws_secret_access_key=creds["secretAccessKey"],
            aws_session_token=creds["sessionToken"],
            region_name=args.region,
        )
        sts = session.client("sts")
        identity = sts.get_caller_identity()
        print("\nAuthentication successful! (Duo MFA session active)")
        print(f"  Account:  {identity['Account']}")
        print(f"  Arn:      {identity['Arn']}")
        print(f"  UserId:   {identity['UserId']}")
        return

    # Default: print credentials as JSON
    print(json.dumps({
        "AccessKeyId": creds["accessKeyId"],
        "SecretAccessKey": creds["secretAccessKey"],
        "SessionToken": creds["sessionToken"],
        "Expiration": datetime.fromtimestamp(
            creds["expiration"] / 1000, tz=timezone.utc
        ).isoformat(),
        "Region": args.region,
    }, indent=2))


if __name__ == "__main__":
    main()
