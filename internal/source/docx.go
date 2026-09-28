package source

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
)

var headingStyle = regexp.MustCompile(`(?i)^heading\s*(\d)$`)

// parseDOCX converts a Word document to Markdown. Heading styles are
// resolved through styles.xml, whose style names are language independent
// ("heading 1" even in a Chinese Word), or through outline levels.
func parseDOCX(name string, data []byte) ([]doc, error) {
	z, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("not a readable Word file: %w", err)
	}
	read := func(entry string) ([]byte, error) {
		for _, f := range z.File {
			if f.Name == entry {
				r, err := f.Open()
				if err != nil {
					return nil, err
				}
				defer r.Close()
				return io.ReadAll(io.LimitReader(r, 64<<20))
			}
		}
		return nil, fmt.Errorf("Word file has no %s", entry)
	}
	levels := map[string]int{}
	if styles, err := read("word/styles.xml"); err == nil {
		var s struct {
			Styles []struct {
				ID   string `xml:"styleId,attr"`
				Name struct {
					Val string `xml:"val,attr"`
				} `xml:"name"`
				Outline struct {
					Val string `xml:"val,attr"`
				} `xml:"pPr>outlineLvl"`
			} `xml:"style"`
		}
		if xml.Unmarshal(styles, &s) == nil {
			for _, st := range s.Styles {
				if m := headingStyle.FindStringSubmatch(st.Name.Val); m != nil {
					levels[st.ID], _ = strconv.Atoi(m[1])
				} else if strings.EqualFold(st.Name.Val, "title") {
					levels[st.ID] = 1
				} else if n, err := strconv.Atoi(st.Outline.Val); err == nil && st.Outline.Val != "" {
					levels[st.ID] = n + 1
				}
			}
		}
	}
	body, err := read("word/document.xml")
	if err != nil {
		return nil, err
	}
	var out strings.Builder
	d := xml.NewDecoder(bytes.NewReader(body))
	var para strings.Builder
	style, outline := "", -1
	var row []string
	inCell, inTable := false, false
	tableRows := 0
	flush := func() {
		text := strings.TrimSpace(para.String())
		para.Reset()
		if inCell {
			row = append(row, strings.ReplaceAll(text, "|", "\\|"))
			return
		}
		if text == "" {
			return
		}
		level := levels[style]
		if outline >= 0 && level == 0 {
			level = outline + 1
		}
		if level > 0 && level <= 6 {
			out.WriteString(strings.Repeat("#", level) + " " + text + "\n\n")
		} else {
			out.WriteString(text + "\n\n")
		}
	}
	for {
		tok, err := d.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("Word document: %w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "p":
				style, outline = "", -1
			case "pStyle":
				style = xmlAttr(t, "val")
			case "outlineLvl":
				if n, err := strconv.Atoi(xmlAttr(t, "val")); err == nil {
					outline = n
				}
			case "tab":
				para.WriteString("\t")
			case "br":
				para.WriteString(" ")
			case "tbl":
				inTable, tableRows = true, 0
			case "tr":
				row = nil
			case "tc":
				inCell = true
			}
		case xml.CharData:
			para.Write(t)
		case xml.EndElement:
			switch t.Name.Local {
			case "p":
				flush()
			case "tc":
				inCell = false
			case "tr":
				out.WriteString("| " + strings.Join(row, " | ") + " |\n")
				if tableRows == 0 {
					out.WriteString("|" + strings.Repeat(" --- |", len(row)) + "\n")
				}
				tableRows++
			case "tbl":
				if inTable {
					out.WriteString("\n")
				}
				inTable = false
			}
		}
	}
	return []doc{markdownDoc(name, out.String())}, nil
}

func xmlAttr(t xml.StartElement, local string) string {
	for _, a := range t.Attr {
		if a.Name.Local == local {
			return a.Value
		}
	}
	return ""
}

// parseNotebook converts a Jupyter notebook: Markdown cells as they are,
// code cells as fenced code in the kernel's language, text outputs after.
func parseNotebook(name string, data []byte) ([]doc, error) {
	var nb struct {
		Metadata struct {
			Kernel struct {
				Language string `json:"language"`
			} `json:"kernelspec"`
			Language struct {
				Name string `json:"name"`
			} `json:"language_info"`
		} `json:"metadata"`
		Cells []struct {
			Type    string          `json:"cell_type"`
			Source  json.RawMessage `json:"source"`
			Outputs []struct {
				Text json.RawMessage `json:"text"`
				Data struct {
					Plain json.RawMessage `json:"text/plain"`
				} `json:"data"`
			} `json:"outputs"`
		} `json:"cells"`
	}
	if err := json.Unmarshal(data, &nb); err != nil {
		return nil, fmt.Errorf("not a readable notebook: %w", err)
	}
	lang := nb.Metadata.Language.Name
	if lang == "" {
		lang = nb.Metadata.Kernel.Language
	}
	var out strings.Builder
	for _, c := range nb.Cells {
		src := strings.TrimRight(jsonText(c.Source), "\n")
		switch c.Type {
		case "markdown":
			out.WriteString(src + "\n\n")
		case "code":
			if src == "" {
				continue
			}
			out.WriteString("```" + lang + "\n" + src + "\n```\n\n")
			for _, o := range c.Outputs {
				text := jsonText(o.Text)
				if text == "" {
					text = jsonText(o.Data.Plain)
				}
				if text = strings.TrimRight(text, "\n"); text != "" {
					out.WriteString("```\n" + text + "\n```\n\n")
				}
			}
		}
	}
	return []doc{markdownDoc(name, out.String())}, nil
}

// jsonText reads a notebook string field, stored as a string or a list of lines.
func jsonText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var lines []string
	if json.Unmarshal(raw, &lines) == nil {
		return strings.Join(lines, "")
	}
	return ""
}
