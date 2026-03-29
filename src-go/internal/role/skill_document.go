package role

import (
	"bufio"
	"strings"

	"gopkg.in/yaml.v3"
)

type skillFrontmatter struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	Requires    []string `yaml:"requires"`
	Tools       []string `yaml:"tools"`
}

type skillDocument struct {
	Frontmatter skillFrontmatter
	Body        string
}

func parseSkillDocument(content string) skillDocument {
	scanner := bufio.NewScanner(strings.NewReader(content))
	if !scanner.Scan() {
		return skillDocument{}
	}
	if strings.TrimSpace(scanner.Text()) != "---" {
		return skillDocument{Body: strings.TrimSpace(content)}
	}

	var frontmatterLines []string
	var bodyLines []string
	frontmatterClosed := false
	for scanner.Scan() {
		line := scanner.Text()
		if !frontmatterClosed {
			if strings.TrimSpace(line) == "---" {
				frontmatterClosed = true
				continue
			}
			frontmatterLines = append(frontmatterLines, line)
			continue
		}
		bodyLines = append(bodyLines, line)
	}

	document := skillDocument{
		Body: strings.TrimSpace(strings.Join(bodyLines, "\n")),
	}
	if len(frontmatterLines) == 0 {
		return document
	}
	if err := yaml.Unmarshal([]byte(strings.Join(frontmatterLines, "\n")), &document.Frontmatter); err != nil {
		return skillDocument{Body: document.Body}
	}
	return document
}
