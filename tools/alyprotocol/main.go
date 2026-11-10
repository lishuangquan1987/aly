package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	schemaRelPath   = "spec/aly-protocol.schema.json"
	configRelPath   = "tools/alyprotocol/csharp.json"
	tsConfigRelPath = "tools/alyprotocol/ts.json"
	dtoRelPath      = "client/aly-client/model/dto.go"
	fixturesRelPath = "spec/fixtures/scenarios"
	outputRelPath   = "client/aly-client-sdk/AlyClient.CSharpSDK/AlyClient.CSharpSDK/Models/Generated"
	tsOutputRelPath = "client/aly-client-sdk/AlyClient.TypeScriptSDK/src/models.ts"
)

func main() {
	rootFlag := flag.String("root", "", "仓库根目录（默认从当前目录向上查找 "+schemaRelPath+"）")
	writeFlag := flag.Bool("write", false, "生成并写入 C# 模型")
	checkFlag := flag.Bool("check", false, "只校验生成物与 schema 一致，不写盘")
	flag.Usage = usage
	flag.Parse()

	if *writeFlag == *checkFlag {
		usage()
		os.Exit(2)
	}
	root, err := findRoot(*rootFlag)
	if err != nil {
		fail(err)
	}
	if err := process(root, *checkFlag); err != nil {
		fail(err)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `用法：go run . -check | -write [-root <仓库根>]

  -check    校验 schema ↔ dto.go 漂移，并比对 Models/Generated 与生成器输出
  -write    重新生成 C# 模型并清理过期的 .g.cs
`)
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "alyprotocol: "+err.Error())
	os.Exit(1)
}

func findRoot(explicit string) (string, error) {
	check := func(dir string) bool {
		_, err := os.Stat(filepath.Join(dir, schemaRelPath))
		return err == nil
	}
	if explicit != "" {
		abs, err := filepath.Abs(explicit)
		if err != nil {
			return "", fmt.Errorf("解析 -root: %v", err)
		}
		if !check(abs) {
			return "", fmt.Errorf("-root %s 下找不到 %s", abs, schemaRelPath)
		}
		return abs, nil
	}
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("获取当前目录: %v", err)
	}
	for i := 0; i < 12; i++ {
		if check(dir) {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("向上未找到 %s，请用 -root 指定仓库根", schemaRelPath)
}

// process 是 -check 与 -write 的公共流程：解析 → 自校验 → 漂移校验 → 渲染 → 比对/写盘。
func process(root string, checkOnly bool) error {
	schemaData, err := os.ReadFile(filepath.Join(root, schemaRelPath))
	if err != nil {
		return fmt.Errorf("读取 schema: %v", err)
	}
	schema, err := ParseSchema(schemaData)
	if err != nil {
		return err
	}
	cfg, err := LoadConfig(filepath.Join(root, configRelPath))
	if err != nil {
		return err
	}
	dtoData, err := os.ReadFile(filepath.Join(root, dtoRelPath))
	if err != nil {
		return fmt.Errorf("读取 dto.go: %v", err)
	}
	structs, err := ParseGoStructs(dtoData)
	if err != nil {
		return err
	}
	if err := validateCoverage(schema, cfg); err != nil {
		return err
	}
	if errs := Verify(schema, cfg, structs); len(errs) > 0 {
		return fmt.Errorf("协议漂移（schema ↔ dto.go）：\n%s", joinErrors(errs))
	}
	fixtures, err := LoadScenarios(filepath.Join(root, fixturesRelPath))
	if err != nil {
		return err
	}
	var fixtureErrs []error
	for _, sc := range fixtures {
		fixtureErrs = append(fixtureErrs, ValidateScenario(schema, sc)...)
	}
	fixtureErrs = append(fixtureErrs, ValidateFixtureCoverage(fixtures)...)
	if len(fixtureErrs) > 0 {
		return fmt.Errorf("fixtures 校验失败：\n%s", joinErrors(fixtureErrs))
	}
	fmt.Printf("fixtures 校验通过：%d 个场景\n", len(fixtures))
	files, err := Render(schema, cfg)
	if err != nil {
		return err
	}
	tsCfg, err := LoadTSConfig(filepath.Join(root, tsConfigRelPath))
	if err != nil {
		return err
	}
	tsFile, err := RenderTS(schema, tsCfg)
	if err != nil {
		return err
	}
	outDir := filepath.Join(root, outputRelPath)
	tsPath := filepath.Join(root, tsOutputRelPath)
	if checkOnly {
		if err := checkGeneratedFile(tsPath, tsFile); err != nil {
			return err
		}
		return checkFiles(outDir, files)
	}
	if err := writeGeneratedFile(tsPath, tsFile); err != nil {
		return err
	}
	return writeFiles(outDir, files)
}

func checkFiles(outDir string, expected []GeneratedFile) error {
	var missing, mismatch, unexpected []string

	want := make(map[string][]byte, len(expected))
	for _, f := range expected {
		want[f.Name] = f.Content
		path := filepath.Join(outDir, f.Name)
		got, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				missing = append(missing, f.Name)
				continue
			}
			return fmt.Errorf("读取 %s: %v", path, err)
		}
		if !bytes.Equal(normalize(got), normalize(f.Content)) {
			mismatch = append(mismatch, f.Name)
		}
	}

	entries, err := os.ReadDir(outDir)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("读取生成目录: %v", err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".g.cs") {
			continue
		}
		if _, ok := want[e.Name()]; !ok {
			unexpected = append(unexpected, e.Name())
		}
	}

	if len(missing)+len(mismatch)+len(unexpected) == 0 {
		fmt.Printf("校验通过：%d 个生成文件与 schema 一致\n", len(expected))
		return nil
	}
	sort.Strings(missing)
	sort.Strings(mismatch)
	sort.Strings(unexpected)
	var b strings.Builder
	b.WriteString("生成物与 schema 不一致，请运行：cd tools/alyprotocol && go run . -write")
	if len(missing) > 0 {
		b.WriteString("\n缺失：\n  " + strings.Join(missing, "\n  "))
	}
	if len(mismatch) > 0 {
		b.WriteString("\n内容过期：\n  " + strings.Join(mismatch, "\n  "))
	}
	if len(unexpected) > 0 {
		b.WriteString("\n多余：\n  " + strings.Join(unexpected, "\n  "))
	}
	return fmt.Errorf("%s", b.String())
}

func checkGeneratedFile(path string, f GeneratedFile) error {
	got, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("生成文件缺失：%s（请运行：cd tools/alyprotocol && go run . -write）", path)
		}
		return fmt.Errorf("读取 %s: %v", path, err)
	}
	if !bytes.Equal(normalize(got), normalize(f.Content)) {
		return fmt.Errorf("生成文件与 schema 不一致：%s（请运行：cd tools/alyprotocol && go run . -write）", path)
	}
	return nil
}

func writeGeneratedFile(path string, f GeneratedFile) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("创建目录 %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, f.Content, 0o644); err != nil {
		return fmt.Errorf("写入 %s: %v", path, err)
	}
	return nil
}

func writeFiles(outDir string, files []GeneratedFile) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("创建生成目录: %v", err)
	}

	want := make(map[string]bool, len(files))
	for _, f := range files {
		if err := os.WriteFile(filepath.Join(outDir, f.Name), f.Content, 0o644); err != nil {
			return fmt.Errorf("写入 %s: %v", f.Name, err)
		}
		want[f.Name] = true
	}

	entries, err := os.ReadDir(outDir)
	if err != nil {
		return fmt.Errorf("读取生成目录: %v", err)
	}
	removed := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".g.cs") || want[e.Name()] {
			continue
		}
		if err := os.Remove(filepath.Join(outDir, e.Name())); err != nil {
			return fmt.Errorf("删除过期文件 %s: %v", e.Name(), err)
		}
		removed++
	}
	fmt.Printf("已生成 %d 个文件，删除 %d 个过期文件\n", len(files), removed)
	return nil
}

// normalize 消除 CRLF 与 UTF-8 BOM 差异，避免跨平台误报。
func normalize(data []byte) []byte {
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
	return bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))
}

func joinErrors(errs []error) string {
	lines := make([]string, len(errs))
	for i, e := range errs {
		lines[i] = "  - " + e.Error()
	}
	return strings.Join(lines, "\n")
}

// validateCoverage 防止 schema 新增定义后"没人管"：
// 每个定义都必须登记漂移映射，每个生成类型都必须有漂移映射。
func validateCoverage(s *Schema, cfg *Config) error {
	for _, defName := range s.DefNames() {
		if _, ok := cfg.Go.Structs[defName]; !ok {
			return fmt.Errorf("schema 定义 %q 未在 csharp.json 的 go.structs 中登记，字段漂移无法校验", defName)
		}
	}
	for defName := range cfg.Types {
		if _, ok := cfg.Go.Structs[defName]; !ok {
			return fmt.Errorf("生成类型 %q 未在 csharp.json 的 go.structs 中登记，字段漂移无法校验", defName)
		}
	}
	return nil
}
