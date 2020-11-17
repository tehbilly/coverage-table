package main

import (
	"bufio"
	"errors"
	"fmt"
	"github.com/olekukonko/tablewriter"
	"golang.org/x/mod/modfile"
	"golang.org/x/tools/cover"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

func main() {
	path := ""
	if len(os.Args) == 2 {
		path = os.Args[1]
	}

	files, err := findGoFiles(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Unable to walk path for go files:", err)
		os.Exit(1)
	}

	modName, err := modulePath(path)
	if err != nil {
		abs, _ := filepath.Abs(path)
		fmt.Fprintf(os.Stderr, "Unable to find go mod file in %s: %s\n", abs, err)
		os.Exit(1)
	}
	baseName := filepath.Base(modName)

	// We want to store coverage results in a temporary file so we're not cluttering things up
	f, err := ioutil.TempFile("", fmt.Sprintf("%s-*.out", baseName))
	if err != nil {
		fmt.Fprintln(os.Stderr, "Unable to create temporary file for coverage data:", err)
		os.Exit(1)
	}
	defer os.Remove(f.Name())

	cmd := exec.Command("go", "test", "-coverprofile", f.Name(), "./...")
	cmd.Dir = path
	if err := cmd.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "Unable to run 'go test':", err)
		os.Exit(1)
	}

	if err := printCoverTable(modName, files, f.Name()); err != nil {
		fmt.Fprintln(os.Stderr, "Unable to generate coverage table:", err)
		os.Exit(1)
	}
}

func findGoFiles(path string) (map[string]float64, error) {
	files := make(map[string]float64)

	ap, err := filepath.Abs(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to get absolute path for %s: %s\n", path, err)
		ap = path
	}

	if err := filepath.Walk(ap, func(p string, fi os.FileInfo, err error) error {
		if err != nil {
			fmt.Printf("Prevent panic by handling failure accessing a path: %s\n", err)
			return err
		}

		// Skip hidden directories/files
		if fi.IsDir() && strings.HasPrefix(fi.Name(), ".") {
			return filepath.SkipDir
		}

		// Skip 'testdata' directories, as the go tool will ignore those as well
		if fi.IsDir() && fi.Name() == "testdata" {
			return filepath.SkipDir
		}

		if strings.HasSuffix(fi.Name(), ".go") {
			// Ignore test files
			if strings.HasSuffix(fi.Name(), "_test.go") {
				return nil
			}

			// Skip if this is a file in package main
			file, err := os.Open(p)
			if err != nil { return err }
			scanner := bufio.NewScanner(file)
			for scanner.Scan() {
				if scanner.Text() == "package main" {
					return nil
				}
			}

			// TODO: Also skip go files that only contain interfaces

			// Clean up path to match what
			fp := strings.TrimPrefix(p, ap)
			fp = strings.ReplaceAll(fp, "\\", "/")
			fp = strings.TrimPrefix(fp, "/")

			files[fp] = 0
		}

		return nil
	}); err != nil {
		return nil, err
	}

	return files, nil
}

func printCoverTable(modName string, files map[string]float64, file string) error {
	profiles, err := cover.ParseProfiles(file)
	if err != nil {
		return err
	}

	// Calculate coverage percentage for files in coverage report
	for _, profile := range profiles {
		name := strings.Replace(profile.FileName, modName+"/", "", 1)
		cov := percentCovered(profile)

		if _, ok := files[name]; !ok {
			fmt.Fprintln(os.Stderr, "File not in files map:", name)
			for name, _ := range files {
				fmt.Fprintln(os.Stderr, "File:", name)
			}
			return errors.New("unknown file: " + name)
		}

		files[name] = cov
	}

	// Used for sorted list later
	names := make([]string, 0, len(files))
	// Used for averaging coverage
	var percents []float64

	for n, p := range files {
		// Mocks don't count towards coverage
		if strings.Contains(n, "mocks/") {
			continue
		}

		names = append(names, n)
		percents = append(percents, p)
	}

	// Sort so we can go through the files in lexicographical order
	sort.Strings(names)

	table := tablewriter.NewWriter(os.Stdout)
	table.SetColumnAlignment([]int{tablewriter.ALIGN_LEFT, tablewriter.ALIGN_RIGHT})

	for _, name := range names {
		cov := files[name]
		table.Rich([]string{name, fmt.Sprintf("%.2f", cov)}, colorsForPercent(cov))
	}

	avg := avg(percents)
	table.SetFooterAlignment(tablewriter.ALIGN_RIGHT)
	table.SetFooter([]string{"Total", fmt.Sprintf("%.2f", avg)})
	table.SetFooterColor(colorsForPercent(avg)...)
	table.Render()

	return nil
}

func colorsForPercent(cov float64) []tablewriter.Colors {
	switch {
	// cov == 0 means that there are no tests covering the file at all
	case cov == 0:
		return []tablewriter.Colors{{tablewriter.FgHiRedColor}, {tablewriter.FgHiRedColor}}
	case cov < 40:
		return []tablewriter.Colors{{}, {tablewriter.FgHiRedColor}}
	case cov < 60:
		return []tablewriter.Colors{{}, {tablewriter.FgRedColor}}
	case cov < 80:
		return []tablewriter.Colors{{}, {tablewriter.FgYellowColor}}
	case cov < 90:
		return []tablewriter.Colors{{}, {tablewriter.FgGreenColor}}
	default:
		return []tablewriter.Colors{{}, {tablewriter.FgHiGreenColor}}
	}
}

func modulePath(path string) (string, error) {
	bytes, err := ioutil.ReadFile(filepath.Join(path, "go.mod"))
	if err != nil {
		return "", err
	}

	return modfile.ModulePath(bytes), nil
}

func percentCovered(p *cover.Profile) float64 {
	var total int64
	var covered int64

	for _, block := range p.Blocks {
		total += int64(block.NumStmt)
		if block.Count > 0 {
			covered += int64(block.Count)
		}
	}

	if total == 0 {
		return 0
	}

	return float64(covered) / float64(total) * 100
}

func avg(counts []float64) float64 {
	var sum float64

	for _, count := range counts {
		sum += count
	}

	return sum / float64(len(counts))
}
