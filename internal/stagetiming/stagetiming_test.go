package stagetiming

import "testing"

func TestParseQueueAdvance(t *testing.T) {
	if p := ParseQueueAdvance("Row abc: Printing → Done"); !p.Ok || p.RowKey != "abc" || p.From != "Printing" || p.To != "Done" {
		t.Errorf("row parse = %+v", p)
	}
	if p := ParseQueueAdvance("Created → Printing"); !p.Ok || p.RowKey != "" || p.From != "Created" || p.To != "Printing" {
		t.Errorf("job parse = %+v", p)
	}
	if p := ParseQueueAdvance("no arrow here"); p.Ok {
		t.Errorf("unparseable should be Ok=false, got %+v", p)
	}
}

func TestFoldStage(t *testing.T) {
	if FoldStage("printing") != "Printing" {
		t.Error("case-insensitive default match failed")
	}
	if FoldStage("Fabrication") != OtherStage {
		t.Error("custom stage should fold to OtherStage")
	}
	if FoldStage("  Done  ") != "Done" {
		t.Error("trim + match failed")
	}
}

func TestAgeBucket(t *testing.T) {
	cases := map[float64]string{0: "0-2d", 2.9: "0-2d", 3: "3-7d", 7.9: "3-7d", 8: "8-14d", 14.9: "8-14d", 15: "15+d", 100: "15+d"}
	for d, want := range cases {
		if got := AgeBucket(d); got != want {
			t.Errorf("AgeBucket(%v) = %s, want %s", d, got, want)
		}
	}
}

func TestPercentileAndMedian(t *testing.T) {
	if Percentile(nil, 90) != nil {
		t.Error("empty percentile should be nil")
	}
	one := Percentile([]float64{5}, 90)
	if one == nil || *one != 5 {
		t.Errorf("single-value percentile = %v, want 5", one)
	}
	// median of [1,2,3,4] = interpolate rank 1.5 between 2 and 3 = 2.5
	m := Median([]float64{1, 2, 3, 4})
	if m == nil || *m != 2.5 {
		t.Errorf("median = %v, want 2.5", m)
	}
	// p90 of 1..10 -> rank 8.1 between index 8 (9) and 9 (10) = 9.1
	p := Percentile([]float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, 90)
	if p == nil || *p != 9.1 {
		t.Errorf("p90 = %v, want 9.1", p)
	}
}

func TestRound1(t *testing.T) {
	if Round1(2.34) != 2.3 || Round1(2.35) != 2.4 || Round1(0) != 0 {
		t.Errorf("round1 wrong: %v %v %v", Round1(2.34), Round1(2.35), Round1(0))
	}
}
