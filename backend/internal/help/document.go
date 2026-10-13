package help

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"errors"
	"regexp"
	"strings"
)

//go:embed resources/ANNOTATOR-GUIDE.zh.md
var manual string

type Section struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Anchor string `json:"anchor"`
}
type Document struct {
	Version  string    `json:"documentVersion"`
	SHA256   string    `json:"documentSHA256"`
	Content  string    `json:"content"`
	Sections []Section `json:"sections"`
}

func ParseDocument(content string) (Document, error) {
	if len(content) == 0 || len(content) > 32768 {
		return Document{}, errors.New("manual size is invalid")
	}
	sum := sha256.Sum256([]byte(content))
	d := Document{SHA256: hex.EncodeToString(sum[:]), Content: content, Sections: []Section{}}
	version := regexp.MustCompile(`文档版本[：:]\s*([^。\s]+)`).FindStringSubmatch(content)
	if len(version) != 2 {
		return Document{}, errors.New("manual version missing")
	}
	d.Version = version[1]
	headings := regexp.MustCompile(`(?m)^## ([a-z][a-z0-9_-]*)[：:]\s*(.+)$`).FindAllStringSubmatch(content, -1)
	seen := map[string]bool{}
	for _, h := range headings {
		if seen[h[1]] {
			return Document{}, errors.New("duplicate section")
		}
		seen[h[1]] = true
		d.Sections = append(d.Sections, Section{h[1], strings.TrimSpace(h[2]), "#" + h[1]})
	}
	if len(d.Sections) == 0 {
		return Document{}, errors.New("manual sections missing")
	}
	return d, nil
}
func EmbeddedDocument() (Document, error) { return ParseDocument(manual) }
