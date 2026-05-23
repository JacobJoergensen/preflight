package rust

import "testing"

func TestAdvisorySeverity(t *testing.T) {
	tests := []struct {
		name          string
		informational string
		cvss          string
		want          string
	}{
		{
			name: "high-impact vector is critical",
			cvss: "CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H",
			want: "critical",
		},
		{
			name: "changed scope pushes the score to critical",
			cvss: "CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:C/C:H/I:H/A:H",
			want: "critical",
		},
		{
			name: "mid-range vector is moderate",
			cvss: "CVSS:3.1/AV:L/AC:L/PR:L/UI:N/S:U/C:H/I:N/A:N",
			want: "moderate",
		},
		{
			name:          "informational advisory is info even with a high cvss",
			informational: "unmaintained",
			cvss:          "CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H",
			want:          "info",
		},
		{
			name: "absent cvss is info",
			want: "info",
		},
		{
			name: "unparseable vector is info",
			cvss: "not-a-cvss-vector",
			want: "info",
		},
		{
			name: "zero-impact vector is info",
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
