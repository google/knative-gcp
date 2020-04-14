package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"text/template"
)

func main() {
	fileNameFlag := flag.String("file", "", "The schema to generate")

	flag.Parse()

	wd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot get working directory: %v\n", err)
		os.Exit(1)
	}

	fileName := *fileNameFlag
	if fileName == "" {
		fmt.Println("--file must be specified")
		os.Exit(1)
	}

	if !strings.HasPrefix(fileName, "/") {
		fileName = wd + fileName
	}
	b, err := ioutil.ReadFile(fileName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to read file %q, %v\n", fileName, err)
		os.Exit(1)
	}
	contents := string(b)

	snippets, err := readInSnippets(wd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to read snippets: %v\n", err)
		os.Exit(1)
	}

	tmpl, err := template.New("").Funcs(funcMap(snippets)).Parse(contents)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to parse contents of file: %v\n%v\n", err, contents)
		os.Exit(1)
	}

	var w bytes.Buffer
	if err := tmpl.Execute(&w, snippets); err != nil {
		fmt.Fprintf(os.Stderr, "Unable to execute template: %v\n", err)
		os.Exit(1)
	}

	fmt.Print(w.String())
}

func funcMap(snippets map[string]string) template.FuncMap {
	return template.FuncMap{
		"replace_with": func(numSpaces int, file string) (string, error) {
			v, present := snippets[file]
			if !present {
				return "", fmt.Errorf("unable to find %q", file)
			}

			spaces := "\n" + strings.Repeat(" ", numSpaces)
			sb := strings.Builder{}
			for _, line := range strings.Split(v, "\n") {
				// Skip blank lines.
				if strings.TrimSpace(line) == "" {
					continue
				}
				if _, err := sb.WriteString(spaces); err != nil {
					return "", fmt.Errorf("unable to write spaces: %v", err)
				}
				if _, err := sb.WriteString(line); err != nil {
					return "", fmt.Errorf("unable to write line: %v", err)
				}
			}
			return sb.String(), nil
		},
	}
}

func readInSnippets(wd string) (map[string]string, error) {
	dir := wd + "/hack/crdgeneration/snippets/"
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	snippet := make(map[string]string)
	for _, f := range files {
		if !strings.HasSuffix(f.Name(), ".yaml") {
			continue
		}
		b, err := ioutil.ReadFile(dir + f.Name())
		if err != nil {
			return nil, err
		}
		contents := string(b)

		fileNameWithoutExtension := strings.TrimSuffix(f.Name(), ".yaml")
		snippet[fileNameWithoutExtension] = contents
	}
	return preprocessSnippet(snippet)
}

func preprocessSnippet(snippets map[string]string) (map[string]string, error) {
	changed := true
	for changed {
		changed = false
		newSnippets := make(map[string]string, len(snippets))

		tmpl := template.New("").Funcs(funcMap(snippets))
		for n, v := range snippets {
			t, err := tmpl.Parse(v)
			if err != nil {
				return nil, fmt.Errorf("unable to parse template %v, %q", err, v)
			}
			var w bytes.Buffer
			if err := t.Execute(&w, snippets); err != nil {
				return nil, fmt.Errorf("unable to execute template: %v", err)
			}
			newSnippets[n] = w.String()
			if newSnippets[n] != snippets[n] {
				changed = true
			}
		}
		snippets = newSnippets
	}
	return snippets, nil
}
