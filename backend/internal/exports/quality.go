package exports

import (
	"context"
	"sort"

	"github.com/XB-Dong/classroom-annotator/backend/internal/domain"
)

const QualityRulesVersion = "P2-QC-1"
const MaxQualityFindings = 100
const MaxQualityPairs = 100000

type QualityFinding struct {
	Code          string   `json:"code"`
	Severity      string   `json:"severity"`
	FrameID       string   `json:"frameId"`
	AnnotationIDs []string `json:"annotationIds"`
	Message       string   `json:"message"`
}

type QualityFrameRevision struct {
	FrameID  string `json:"frameId"`
	Revision int64  `json:"revision"`
}

type QualityReport struct {
	RulesVersion       string                 `json:"rulesVersion"`
	CheckedFrames      int                    `json:"checkedFrames"`
	CheckedAnnotations int                    `json:"checkedAnnotations"`
	TotalFindings      int                    `json:"totalFindings"`
	Truncated          bool                   `json:"truncated"`
	FrameRevisions     []QualityFrameRevision `json:"frameRevisions"`
	Findings           []QualityFinding       `json:"findings"`
}

// CheckQuality evaluates an already validated export snapshot without changing it.
// Counts describe the captured snapshot; truncated means the scan was incomplete.
func CheckQuality(ctx context.Context, records []Record) (QualityReport, error) {
	report := QualityReport{RulesVersion: QualityRulesVersion, CheckedFrames: len(records), FrameRevisions: []QualityFrameRevision{}, Findings: []QualityFinding{}}
	if err := ctx.Err(); err != nil {
		return QualityReport{}, err
	}
	ordered := append([]Record(nil), records...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].Frame.ID < ordered[j].Frame.ID })
	for _, record := range ordered {
		report.CheckedAnnotations += len(record.Annotations)
		report.FrameRevisions = append(report.FrameRevisions, QualityFrameRevision{FrameID: record.Frame.ID, Revision: record.Frame.Revision})
	}
	add := func(frame, code, message string, ids ...string) bool {
		report.TotalFindings++
		if len(report.Findings) == MaxQualityFindings {
			report.Truncated = true
			return false
		}
		report.Findings = append(report.Findings, QualityFinding{Code: code, Severity: "warning", FrameID: frame, AnnotationIDs: ids, Message: message})
		return true
	}
	pairs := 0
	for _, record := range ordered {
		if err := ctx.Err(); err != nil {
			return QualityReport{}, err
		}
		anns := append([]domain.Annotation(nil), record.Annotations...)
		sort.Slice(anns, func(i, j int) bool { return anns[i].ID < anns[j].ID })
		// Evaluate in frame/code/annotation order so capped output is repeatable.
	duplicates:
		for i := 0; i < len(anns); i++ {
			for j := i + 1; j < len(anns); j++ {
				if err := ctx.Err(); err != nil {
					return QualityReport{}, err
				}
				if pairs == MaxQualityPairs {
					report.Truncated = true
					break duplicates
				}
				pairs++
				if qualityIoU(anns[i].BBox, anns[j].BBox) >= .90 {
					if !add(record.Frame.ID, "DUPLICATE_BOX", "Boxes overlap by at least 90%; check whether both labels are needed.", anns[i].ID, anns[j].ID) {
						return report, nil
					}
				}
			}
		}
		for _, rule := range []string{"LARGE_BOX", "TINY_BOX"} {
			for _, ann := range anns {
				if err := ctx.Err(); err != nil {
					return QualityReport{}, err
				}
				b := ann.BBox
				if rule == "LARGE_BOX" && b[2]*b[3] > float64(record.Frame.Width)*float64(record.Frame.Height)*.25 {
					if !add(record.Frame.ID, rule, "Box covers more than 25% of the image; check the visible head boundary.", ann.ID) {
						return report, nil
					}
				}
				if rule == "TINY_BOX" && min(b[2], b[3]) < 4 {
					if !add(record.Frame.ID, rule, "Box has a side smaller than 4 pixels; check the visible head boundary.", ann.ID) {
						return report, nil
					}
				}
			}
		}
	}
	return report, ctx.Err()
}

func qualityIoU(a, b [4]float64) float64 {
	w := min(a[0]+a[2], b[0]+b[2]) - max(a[0], b[0])
	h := min(a[1]+a[3], b[1]+b[3]) - max(a[1], b[1])
	if w <= 0 || h <= 0 {
		return 0
	}
	intersection := w * h
	return intersection / (a[2]*a[3] + b[2]*b[3] - intersection)
}
