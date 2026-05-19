package adapter

import "testing"

func TestAdvisorySeverity(t *testing.T) {
	tests := []struct {
		name          string
		informational string
		cvss          string
		want          string
	}{
		{
			name: "critical CVSS v3.1 vector scores 9.8",
			cvss: "CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H",
			want: "critical",
		},
		{
			name: "moderate CVSS v3.1 vector scores 5.3",
			cvss: "CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:N/I:N/A:L",
			want: "moderate",
		},
		{
			name: "low CVSS v3.1 vector scores 2.9",
			cvss: "CVSS:3.1/AV:L/AC:H/PR:H/UI:R/S:U/C:L/I:L/A:N",
			want: "low",
		},
		{
			name: "high CVSS v3.1 vector with scope change",
			cvss: "CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:C/C:L/I:L/A:N",
			want: "high",
		},
		{
			name: "CVSS v3.0 prefix accepted",
			cvss: "CVSS:3.0/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H",
			want: "critical",
		},
		{
			name:          "informational advisory uses info bucket",
			informational: "unmaintained",
			cvss:          "CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H",
			want:          "info",
		},
		{
			name: "missing CVSS falls back to info",
			cvss: "",
			want: "info",
		},
		{
			name: "CVSS v2 vector unsupported",
			cvss: "AV:N/AC:L/Au:N/C:P/I:P/A:P",
			want: "info",
		},
		{
			name: "malformed vector unsupported",
			cvss: "CVSS:3.1/garbage",
			want: "info",
		},
		{
			name: "zero-impact vector falls into info",
			cvss: "CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:N/I:N/A:N",
			want: "info",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := advisorySeverity(tt.informational, tt.cvss)

			if got != tt.want {
				t.Errorf("advisorySeverity(%q, %q) = %q, want %q", tt.informational, tt.cvss, got, tt.want)
			}
		})
	}
}
