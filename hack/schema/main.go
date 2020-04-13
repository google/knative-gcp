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
		fmt.Printf("cannot get working directory: %v\n", err)
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
		fmt.Printf("Unable to read file %q, %v\n", fileName, err)
		os.Exit(1)
	}
	contents := string(b)

	generics, err := readInGenerics(wd)
	if err != nil {
		fmt.Printf("Unable to read generics: %v\n", err)
		os.Exit(1)
	}

	tmpl, err := template.New("").Funcs(funcMap(generics)).Parse(contents)
	if err != nil {
		fmt.Printf("Unable to parse contents of file: %v\n%v\n", err, contents)
		os.Exit(1)
	}

	var w bytes.Buffer
	if err := tmpl.Execute(&w, generics); err != nil {
		fmt.Printf("Unable to execute template: %v\n", err)
		os.Exit(1)
	}

	fmt.Print(w.String())
}

func funcMap(generics map[string]string) template.FuncMap {
	return template.FuncMap{
		"replace_with": func(numSpaces int, file string) (string, error) {
			v, present := generics[file]
			if !present {
				return "", fmt.Errorf("unable to find %q", file)
			}

			spaces := "\n" + strings.Repeat(" ", numSpaces)
			sb := strings.Builder{}
			for _, line := range strings.Split(v, "\n") {
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

func readInGenerics(wd string) (map[string]string, error) {
	dir := wd + "/hack/schema/"
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	generic := make(map[string]string)
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
		generic[fileNameWithoutExtension] = contents
	}
	return preprocessGeneric(generic)
}

func preprocessGeneric(generics map[string]string) (map[string]string, error) {
	changed := true
	for changed {
		changed = false
		newGenerics := make(map[string]string, len(generics))

		tmpl := template.New("").Funcs(funcMap(generics))
		for n, v := range generics {
			t, err := tmpl.Parse(v)
			if err != nil {
				return nil, fmt.Errorf("unable to parse template %v, %q", err, v)
			}
			var w bytes.Buffer
			if err := t.Execute(&w, generics); err != nil {
				return nil, fmt.Errorf("unable to execute template: %v", err)
			}
			newGenerics[n] = w.String()
			if newGenerics[n] != generics[n] {
				changed = true
			}
		}
		generics = newGenerics
	}
	return generics, nil
}
