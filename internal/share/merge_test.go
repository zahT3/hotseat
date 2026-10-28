package share

import (
	"strings"
	"testing"
)

func TestMergeJSONLKeepsMalformedLines(t *testing.T) {
	core := `{"display":"a","timestamp":2}` + "\n" + `not json`
	acct := "{\"display\":\"b\",\"timestamp\":1}\r\n{\"display\":\"a\",\"timestamp\":2}\n{\"display\":\"c\"}\n"
	got := string(mergeJSONL([]byte(core), []byte(acct)))
	want := strings.Join([]string{
		`not json`,
		`{"display":"c"}`,
		`{"display":"b","timestamp":1}`,
		`{"display":"a","timestamp":2}`,
	}, "\n") + "\n"
	if got != want {
		t.Errorf("got:\n%q\nwant:\n%q", got, want)
	}
}

func TestMergeJSONLEmptyInputs(t *testing.T) {
	if got := string(mergeJSONL(nil, []byte("\n\n"))); got != "" {
		t.Errorf("got %q, want empty", got)
	}
}
