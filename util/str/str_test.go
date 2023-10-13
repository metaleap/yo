package str

import "testing"

func TestInterp(t *testing.T) {
	for s, m := range map[string]map[string]string{
		"foo bar":                   {},
		"foo{bar}baz":               {"bar": "BAAAR", "not_exist": "so it goes"},
		"{oneOnly}":                 {"oneOnly": "fullStr"},
		"{two}{together}":           {"two": "Zwei", "together": "Zusammen"},
		"many{r1}Are{r2}In{r3}Here": {"r1": "(Eins)", "r2": "(Zwei)", "r3": "(Drei)"},
		"{start}IsFine":             {"start": "THIS:"},
		"ThisIs{end}":               {"end": ":FINE"},
	} {
		t.Log(">>>" + Interp(s, m) + "<<<")
	}
}
