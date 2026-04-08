{{/*
Expand the name of the chart.
*/}}
{{- define "agentops-operator.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "agentops-operator.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "agentops-operator.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "agentops-operator.labels" -}}
helm.sh/chart: {{ include "agentops-operator.chart" . }}
{{ include "agentops-operator.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/component: operator
{{- end }}

{{/*
Selector labels
*/}}
{{- define "agentops-operator.selectorLabels" -}}
app.kubernetes.io/name: {{ include "agentops-operator.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
control-plane: controller-manager
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "agentops-operator.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "agentops-operator.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Create the name of the leader election role
*/}}
{{- define "agentops-operator.leaderElectionRoleName" -}}
{{- printf "%s-leader-election" (include "agentops-operator.fullname" .) }}
{{- end }}

{{/*
Create the leader election ID
*/}}
{{- define "agentops-operator.leaderElectionId" -}}
{{- .Values.leaderElection.id | default "e58828d7.agentops.io" }}
{{- end }}

{{/*
Operator image
*/}}
{{- define "agentops-operator.image" -}}
{{- if .Values.image.digest }}
{{- printf "%s@%s" .Values.image.repository .Values.image.digest }}
{{- else }}
{{- printf "%s:%s" .Values.image.repository (.Values.image.tag | default .Chart.AppVersion) }}
{{- end }}
{{- end }}
