package preannotation

import "github.com/XB-Dong/classroom-annotator/backend/internal/domain"

func RiskReasons(detections []Detection, existing []domain.Annotation) [][]string {
	reasons := make([][]string, len(detections))
	for i, d := range detections {
		rs := []string{}
		if d.Confidence < .55 {
			rs = append(rs, "LOW_CONFIDENCE")
		}
		if d.BBox[2] < 12 || d.BBox[3] < 12 {
			rs = append(rs, "SMALL_HEAD")
		}
		if d.Clipped {
			rs = append(rs, "CLIPPED_BOX")
		}
		if d.TileBoundary {
			rs = append(rs, "TILE_BOUNDARY")
		}
		for j, other := range detections {
			if i != j && IoU(d.BBox, other.BBox) >= .35 {
				rs = append(rs, "POSSIBLE_DUPLICATE")
				break
			}
		}
		for _, a := range existing {
			if IoU(d.BBox, a.BBox) >= .35 {
				rs = append(rs, "OVERLAP_EXISTING")
				break
			}
		}
		reasons[i] = rs
	}
	return reasons
}
func IoU(a, b [4]float64) float64 {
	x1 := max(a[0], b[0])
	y1 := max(a[1], b[1])
	x2 := min(a[0]+a[2], b[0]+b[2])
	y2 := min(a[1]+a[3], b[1]+b[3])
	if x2 <= x1 || y2 <= y1 {
		return 0
	}
	intersection := (x2 - x1) * (y2 - y1)
	union := a[2]*a[3] + b[2]*b[3] - intersection
	if union <= 0 {
		return 0
	}
	return intersection / union
}
