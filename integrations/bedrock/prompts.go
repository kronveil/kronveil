package bedrock

import (
	"bytes"
	"text/template"
)

// Prompt templates for LLM-powered analysis.
var (
	RootCauseTemplate = template.Must(template.New("rootcause").Parse(`
Analyze this infrastructure incident and determine the root cause.

## Incident Details
- **Service:** {{.Service}}
- **Severity:** {{.Severity}}
- **Time:** {{.Timestamp}}

## Dependency Chain
{{range .Dependencies}}- {{.}}
{{end}}

## Related Events
{{range .Events}}- [{{.Severity}}] {{.Source}}: {{.Type}} at {{.Timestamp}}
  {{range $k, $v := .Payload}}  {{$k}}: {{$v}}
  {{end}}
{{end}}

## Impacted Services
{{range .ImpactedServices}}- {{.}}
{{end}}

Provide your analysis in this format:
1. **Root Cause:** One-sentence summary
2. **Causal Chain:** Step-by-step explanation of how the failure propagated
3. **Evidence:** Key data points supporting your analysis
4. **Remediation:** Recommended fix
5. **Prevention:** How to prevent this in the future
`))

	IncidentSummaryTemplate = template.Must(template.New("summary").Parse(`
Summarize this infrastructure incident for an on-call engineer.

## Incident {{.ID}}
- **Title:** {{.Title}}
- **Severity:** {{.Severity}}
- **Status:** {{.Status}}
- **Created:** {{.CreatedAt}}
- **Affected Resources:** {{range .AffectedResources}}{{.}}, {{end}}

## Timeline
{{range .Timeline}}- [{{.Timestamp}}] {{.Action}}: {{.Details}} ({{.Actor}})
{{end}}

Provide a concise 2-3 sentence summary suitable for a Slack notification.
`))

	CapacityTemplate = template.Must(template.New("capacity").Parse(`
Analyze the following resource utilization data and provide capacity planning recommendations.

## Resource: {{.Resource}}
- **Current Utilization:** {{.CurrentValue}}%
- **Trend:** {{.Trend}}
- **Forecast Horizon:** {{.ForecastDays}} days
{{if .DaysToCapacity}}- **Days to Capacity:** {{.DaysToCapacity}}{{end}}

## Historical Data (last 7 days)
{{range .RecentData}}- {{.Timestamp}}: {{.Value}}%
{{end}}

Provide:
1. Current capacity assessment
2. Forecast for next {{.ForecastDays}} days
3. Right-sizing recommendation (scale up, scale down, or maintain)
4. Cost optimization suggestions
`))

	AnomalyExplanationTemplate = template.Must(template.New("anomaly").Parse(`
Explain this detected anomaly in plain language for an operations engineer.

## Anomaly Details
- **Signal:** {{.Signal}}
- **Anomaly Score:** {{.Score}} (0-1 scale)
- **Current Value:** {{.Value}}
- **Mean:** {{.Mean}}
- **Standard Deviation:** {{.StdDev}}
- **Z-Score:** {{.ZScore}}
- **Predicted:** {{.Predicted}}

Provide a brief (2-3 sentences) explanation of:
1. What happened
2. Why it's unusual
3. What might have caused it
`))
)

// RootCausePromptData holds data for the root cause analysis prompt.
type RootCausePromptData struct {
	Service          string
	Severity         string
	Timestamp        string
	Dependencies     []string
	Events           []EventData
	ImpactedServices []string
}

// EventData holds event information for prompt rendering.
type EventData struct {
	Source    string
	Type     string
	Severity string
	Timestamp string
	Payload  map[string]interface{}
}

// IncidentSummaryData holds data for incident summary prompts.
type IncidentSummaryData struct {
	ID                string
	Title             string
	Severity          string
	Status            string
	CreatedAt         string
	AffectedResources []string
	Timeline          []TimelineData
}

// TimelineData holds timeline entry data for prompts.
type TimelineData struct {
	Timestamp string
	Action    string
	Details   string
	Actor     string
}

// RenderPrompt renders a template with the given data.
func RenderPrompt(tmpl *template.Template, data interface{}) (string, error) {
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}
