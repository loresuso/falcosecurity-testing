package data

import (
	"archive/zip"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/iancoleman/strcase"
	"github.com/sirupsen/logrus"
)

const (
	FalcoSourceCodeURL = "https://github.com/falcosecurity/falco/archive/refs/tags/0.33.1.zip"
)

var DownloadDir = ""

func init() {
	var err error
	DownloadDir, err = filepath.Abs("../../../../generated")
	if err != nil {
		log.Fatal(err.Error())
	}
}

type LargeFileVarInfo struct {
	VarName  string
	FileName string
	FilePath string
}

type StringFileVarInfo struct {
	VarName     string
	FileName    string
	FileContent string
}

type GenTemplateInfo struct {
	Timestamp   time.Time
	PackageName string
	LargeFiles  []*LargeFileVarInfo
	StringFiles []*StringFileVarInfo
}

var genTemplate = template.Must(template.New("getTemplate").Parse(
	`// Code generated by go generate; DO NOT EDIT.
// This file was generated by robots at {{ .Timestamp }}

package {{ .PackageName }}

import (
	"github.com/jasondellaluce/falco-testing/pkg/run"
)
{{range $idx, $info := .StringFiles}}
var {{ .VarName }} = run.NewStringFileAccessor(
	"{{ .FileName }}",
	` + "`" + `{{ .FileContent }}` + "`" + `,
)
{{end}}{{range $idx, $info := .LargeFiles}}
var {{ .VarName }} = run.NewLocalFileAccessor(
	"{{ .FileName }}",
	"{{ .FilePath }}",
)
{{end}}
`))

func GenSourceFile(w io.Writer, info *GenTemplateInfo) error {
	return genTemplate.Execute(w, info)
}

func Download(url, outPath string) error {
	if _, err := os.Stat(outPath); err == nil {
		logrus.Infof("skipping download of %s, %s is already present", url, outPath)
		return nil
	}
	logrus.Infof("creating dir %s", filepath.Base(outPath))
	err := os.MkdirAll(filepath.Dir(outPath), os.ModePerm)
	if err != nil {
		return err
	}
	logrus.Infof("downloading %s into %s", url, outPath)
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	file, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = io.Copy(file, resp.Body)
	if err != nil {
		return err
	}
	return nil
}

func Unzip(zipFile, outDir string) error {
	logrus.Infof("unzipping %s into dir %s", zipFile, outDir)
	reader, err := zip.OpenReader(zipFile)
	if err != nil {
		return err
	}
	defer reader.Close()

	for _, file := range reader.File {
		relname := path.Join(outDir, file.Name)
		if _, err := os.Stat(relname); err == nil {
			logrus.Infof("skipping extraction of %s as it is already present", relname)
			continue
		}

		logrus.Debugf("extracting %s", relname)
		fileReader, err := file.Open()
		if err != nil {
			return err
		}
		defer fileReader.Close()
		if file.FileInfo().IsDir() {
			err = os.MkdirAll(relname, os.ModePerm)
			if err != nil {
				return err
			}
		} else {
			err = os.MkdirAll(path.Dir(relname), os.ModePerm)
			if err != nil {
				return err
			}
			out, err := os.Create(relname)
			if err != nil {
				return err
			}
			defer out.Close()
			_, err = io.Copy(out, fileReader)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func ListDirFiles(dirPath string, recursive bool) ([]string, error) {
	if !strings.HasSuffix(dirPath, "/") {
		dirPath += "/"
	}

	var res []string
	files, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, err
	}
	for _, file := range files {
		fileName := dirPath + file.Name()
		if file.IsDir() {
			if recursive {
				subres, err := ListDirFiles(fileName+"/", recursive)
				if err != nil {
					return nil, err
				}
				res = append(res, subres...)
			}
			continue
		}
		res = append(res, fileName)
	}
	return res, nil
}

func DownloadAndListFalcoCodeFiles() ([]string, error) {
	extractDir := DownloadDir
	err := Download(FalcoSourceCodeURL, DownloadDir+"/falco-code.zip")
	if err != nil {
		return nil, err
	}
	err = Unzip(DownloadDir+"/falco-code.zip", extractDir)
	if err != nil {
		return nil, err
	}
	return ListDirFiles(extractDir+"/falco-0.33.1/", true)
}

func VarNameFromFilePath(path, prefix string) string {
	path = strings.TrimPrefix(path, prefix)
	path = strings.TrimSuffix(path, filepath.Ext(filepath.Base(path)))
	path = strings.ReplaceAll(path, "/", "_")
	return strcase.ToCamel(path)
}