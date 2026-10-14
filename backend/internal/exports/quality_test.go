package exports

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/XB-Dong/classroom-annotator/backend/internal/domain"
)

func qualityRecord(id string, boxes ...[4]float64) Record {
	r := Record{Frame: domain.Frame{ID: id, Width: 100, Height: 100, Revision: 7}}
	for i, box := range boxes {
		r.Annotations = append(r.Annotations, domain.Annotation{ID: fmt.Sprintf("a%04d", i), BBox: box, Source: "manual"})
	}
	return r
}

func TestQualityRuleBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name  string
		boxes [][4]float64
		codes []string
	}{
		{"tiny", [][4]float64{{0, 0, 3.99, 10}}, []string{"TINY_BOX"}},
		{"four pixels", [][4]float64{{0, 0, 4, 10}}, []string{}},
		{"quarter image", [][4]float64{{0, 0, 50, 50}}, []string{}},
		{"large", [][4]float64{{0, 0, 50.01, 50}}, []string{"LARGE_BOX"}},
		{"iou boundary containment", [][4]float64{{0, 0, 10, 10}, {0, 0, 9, 10}}, []string{"DUPLICATE_BOX"}},
		{"below iou", [][4]float64{{0, 0, 10, 10}, {0, 0, 8.99, 10}}, []string{}},
		{"overlap below threshold", [][4]float64{{0, 0, 10, 10}, {2, 0, 10, 10}}, []string{}},
		{"disjoint", [][4]float64{{0, 0, 10, 10}, {20, 0, 10, 10}}, []string{}},
		{"zero heads", nil, []string{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			report, err := CheckQuality(context.Background(), []Record{qualityRecord("f", tc.boxes...)})
			if err != nil {
				t.Fatal(err)
			}
			codes := []string{}
			for _, f := range report.Findings {
				codes = append(codes, f.Code)
				if f.Severity != "warning" || f.FrameID != "f" || f.Message == "" {
					t.Fatal(f)
				}
			}
			if !reflect.DeepEqual(codes, tc.codes) || report.Truncated || report.TotalFindings != len(tc.codes) || report.CheckedFrames != 1 || report.CheckedAnnotations != len(tc.boxes) || report.FrameRevisions[0].Revision != 7 {
				t.Fatal(report)
			}
		})
	}
}

func TestQualityOrderingAndInputImmutability(t *testing.T) {
	a := qualityRecord("a", [4]float64{0, 0, 60, 60}, [4]float64{0, 0, 60, 60}, [4]float64{80, 80, 2, 2})
	b := qualityRecord("b", [4]float64{0, 0, 1, 1})
	first, err := CheckQuality(context.Background(), []Record{a, b})
	if err != nil {
		t.Fatal(err)
	}
	a.Annotations[0], a.Annotations[2] = a.Annotations[2], a.Annotations[0]
	input := []Record{b, a}
	second, err := CheckQuality(context.Background(), input)
	if err != nil || !reflect.DeepEqual(first, second) {
		t.Fatal(first, second, err)
	}
	if input[0].Frame.ID != "b" || a.Annotations[0].ID != "a0002" {
		t.Fatal("input reordered")
	}
	want := []string{"DUPLICATE_BOX", "LARGE_BOX", "LARGE_BOX", "TINY_BOX", "TINY_BOX"}
	for i, code := range want {
		if first.Findings[i].Code != code {
			t.Fatal(first)
		}
	}
}

func TestQualityFindingLimitReportsIncompleteScan(t *testing.T) {
	for _, count := range []int{100, 101} {
		records := make([]Record, count)
		for i := range records {
			records[i] = qualityRecord(fmt.Sprintf("f%04d", i), [4]float64{0, 0, 1, 1})
		}
		report, err := CheckQuality(context.Background(), records)
		if err != nil || len(report.Findings) != 100 || report.TotalFindings != count || report.Truncated != (count > 100) || len(report.FrameRevisions) != count || report.CheckedAnnotations != count {
			t.Fatal(report, err)
		}
	}
}

func TestQualityPairBudgetReportsIncompleteWithoutInventingFindings(t *testing.T) {
	for _, count := range []int{447, 448} {
		r := qualityRecord("a")
		r.Frame.Width = 10000
		for i := 0; i < count; i++ {
			r.Annotations = append(r.Annotations, domain.Annotation{ID: fmt.Sprintf("a%04d", i), BBox: [4]float64{float64(i * 10), 0, 4, 4}, Source: "manual"})
		}
		report, err := CheckQuality(context.Background(), []Record{r, qualityRecord("z", [4]float64{0, 0, 1, 1})})
		if err != nil || report.Truncated != (count == 448) || report.TotalFindings != 1 || report.Findings[0].FrameID != "z" {
			t.Fatal(report, err)
		}
	}
}

func TestQualityContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := CheckQuality(ctx, []Record{qualityRecord("f")}); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	ctx, cancel = context.WithDeadline(context.Background() /* already expired */, time.Unix(1, 0))
	defer cancel()
	if _, err := CheckQuality(ctx, nil); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal(err)
	}
}
