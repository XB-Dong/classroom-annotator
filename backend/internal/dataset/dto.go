package dataset

import (
	"encoding/json"
	"math/big"
	"strconv"
	"strings"
)

type SafeID int64

func (i *SafeID) UnmarshalJSON(b []byte) error {
	if len(b) == 0 || b[0] == '"' {
		return ErrResponse
	}
	v, ok := new(big.Rat).SetString(string(b))
	if !ok || !v.IsInt() || v.Sign() < 0 || v.Num().Cmp(big.NewInt(9007199254740991)) > 0 {
		return ErrResponse
	}
	*i = SafeID(v.Num().Int64())
	return nil
}
func ValidCatalogID(s string) bool {
	if len(s) == 0 || len(s) > 128 || strings.Trim(s, "0123456789") != "" {
		return false
	}
	_, e := strconv.ParseUint(s, 10, 64)
	return e == nil
}

type Video struct {
	ID            SafeID         `json:"id"`
	LessonID      SafeID         `json:"lesson_id"`
	CatalogFileID SafeID         `json:"catalog_file_id"`
	Channel       int            `json:"channel"`
	PartIndex     int            `json:"part_index"`
	Tags          map[string]any `json:"tags,omitempty"`
}
type Lesson struct {
	ID               SafeID  `json:"id"`
	CourseName       string  `json:"course_name"`
	OrganizationName string  `json:"organization_name"`
	Teacher          string  `json:"teacher"`
	Date             string  `json:"date"`
	SectionIndex     int     `json:"section_index"`
	Name             *string `json:"name,omitempty"`
	Room             *string `json:"room,omitempty"`
	StartTime        *string `json:"start_time,omitempty"`
	EndTime          *string `json:"end_time,omitempty"`
	Videos           []Video `json:"videos"`
}

func (v *Video) UnmarshalJSON(b []byte) error {
	type alias Video
	var a alias
	var fields map[string]json.RawMessage
	if json.Unmarshal(b, &fields) != nil {
		return ErrResponse
	}
	for _, k := range []string{"id", "catalog_file_id", "channel"} {
		if _, ok := fields[k]; !ok {
			return ErrResponse
		}
	}
	if e := json.Unmarshal(b, &a); e != nil {
		return e
	}
	*v = Video(a)
	return nil
}
func (l *Lesson) UnmarshalJSON(b []byte) error {
	type alias Lesson
	var a alias
	var fields map[string]json.RawMessage
	if json.Unmarshal(b, &fields) != nil {
		return ErrResponse
	}
	for _, k := range []string{"id", "videos"} {
		if _, ok := fields[k]; !ok {
			return ErrResponse
		}
	}
	if e := json.Unmarshal(b, &a); e != nil {
		return e
	}
	if a.Videos == nil {
		return ErrResponse
	}
	*l = Lesson(a)
	return nil
}

type Page struct {
	Type  string         `json:"type"`
	Items []Lesson       `json:"items"`
	Total int            `json:"total"`
	Stats map[string]any `json:"stats"`
}
