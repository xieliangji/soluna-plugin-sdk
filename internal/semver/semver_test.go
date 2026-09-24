package semver

import "testing"

func TestParseAndCompare(t *testing.T) {
	t.Parallel()
	tests := []struct {
		left  string
		right string
		want  int
	}{
		{left: "1.2.3", right: "1.2.3+build.7", want: 0},
		{left: "1.2.3-alpha", right: "1.2.3", want: -1},
		{left: "1.2.3-alpha.2", right: "1.2.3-alpha.10", want: -1},
		{left: "1.2.3-beta", right: "1.2.3-alpha.9", want: 1},
		{left: "1.2.4-rc.1", right: "1.2.3", want: 1},
		{left: "100000000000000000000.0.0", right: "99999999999999999999.0.0", want: 1},
	}
	for _, test := range tests {
		left, err := Parse(test.left)
		if err != nil {
			t.Fatalf("Parse(%q): %v", test.left, err)
		}
		right, err := Parse(test.right)
		if err != nil {
			t.Fatalf("Parse(%q): %v", test.right, err)
		}
		if got := left.Compare(right); got != test.want {
			t.Errorf("Compare(%q, %q) = %d, want %d", test.left, test.right, got, test.want)
		}
	}
}

func TestParseRejectsMalformedVersions(t *testing.T) {
	t.Parallel()
	for _, value := range []string{"", "1", "1.2", "1.2.3.4", "01.2.3", "1.02.3", "1.2.03", "1.2.3-", "1.2.3-alpha..1", "1.2.3-01", "1.2.3+", "1.2.3+bad_thing", "v1.2.3", "vv1.2.3", " 1.2.3"} {
		if _, err := Parse(value); err == nil {
			t.Errorf("Parse(%q) unexpectedly succeeded", value)
		}
	}
}

func TestParseCommandOutputAcceptsVPrefix(t *testing.T) {
	t.Parallel()
	version, err := ParseCommandOutput("v1.2.3-rc.1")
	minimum, minimumErr := Parse("1.2.3-beta.1")
	if err != nil || minimumErr != nil || version.Compare(minimum) <= 0 {
		t.Fatalf("ParseCommandOutput() = %#v, %v; minimum error = %v", version, err, minimumErr)
	}
}
