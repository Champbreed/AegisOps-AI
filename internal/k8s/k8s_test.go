package k8s

import "testing"

func TestStripCodeFence(t *testing.T) {
	cases := map[string]string{
		"apiVersion: v1":                     "apiVersion: v1",
		"```yaml\napiVersion: v1\n```":       "apiVersion: v1",
		"```\nkind: Deployment\n```\n":       "kind: Deployment",
	}
	for in, want := range cases {
		if got := stripCodeFence(in); got != want {
			t.Errorf("stripCodeFence(%q) = %q, want %q", in, got, want)
		}
	}
}
