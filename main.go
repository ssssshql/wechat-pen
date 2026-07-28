package main

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"wechat-pen/converter"
	"wechat-pen/server"

	"github.com/spf13/cobra"
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "wechat-pen: %v\n", err)
		os.Exit(1)
	}
}

// -- shared flags attached to multiple commands --

type convertFlags struct {
	output       string
	complete     bool
	title        string
	theme        string
	style        string
	primary      string
	textIndent   bool
	justify      bool
	highlight    bool
	toc          bool
	footer       bool
	paragraphGap string
	fontSize     string
	lineHeight   string
	unsafe       bool
	css          string
}

func (f *convertFlags) register(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&f.output, "output", "o", "", "output HTML file (default: stdout, or <input>.html if input is a file)")
	cmd.Flags().BoolVar(&f.complete, "complete", false, "wrap output in a full HTML document")
	cmd.Flags().StringVar(&f.title, "title", "", "document title")
	cmd.Flags().StringVar(&f.theme, "theme", "wechat", "theme: wechat | default")
	cmd.Flags().StringVar(&f.style, "style", "simple", "style pack id (builtin or external)")
	cmd.Flags().StringVar(&f.primary, "primary", "", "primary color hex (default: pack default)")
	cmd.Flags().BoolVar(&f.textIndent, "indent", false, "first-line text indent 2em")
	cmd.Flags().BoolVar(&f.justify, "justify", true, "text justify")
	cmd.Flags().BoolVar(&f.highlight, "highlight", true, "output code block syntax highlighting")
	cmd.Flags().BoolVar(&f.toc, "toc", false, "prepend table of contents from h2/h3")
	cmd.Flags().BoolVar(&f.footer, "footer", false, "append end-of-article footer")
	cmd.Flags().StringVar(&f.paragraphGap, "gap", "1em", "paragraph bottom margin")
	cmd.Flags().StringVar(&f.fontSize, "font-size", "16px", "body font size")
	cmd.Flags().StringVar(&f.lineHeight, "line-height", "1.75", "body line height")
	cmd.Flags().BoolVar(&f.unsafe, "unsafe", false, "allow raw HTML in markdown")
	cmd.Flags().StringVar(&f.css, "css", "", "path to CSS file to embed when using --complete")
}

func (f convertFlags) toConfig(titleOverride string) converter.Config {
	return converter.Config{
		Title:        firstNonEmpty(titleOverride, f.title),
		Theme:        converter.Theme(f.theme),
		Style:        converter.StylePack(f.style),
		PrimaryColor: f.primary,
		TextIndent:   f.textIndent,
		Justify:      f.justify,
		Highlight:    f.highlight,
		TOC:          f.toc,
		Footer:       f.footer,
		ParagraphGap: f.paragraphGap,
		FontSize:     f.fontSize,
		LineHeight:   f.lineHeight,
		ImageCaption: true,
		Complete:     f.complete,
		Unsafe:       f.unsafe,
	}
}

func firstNonEmpty(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}

// -- root command (default: convert single file) --

var rootCmd = &cobra.Command{
	Use:   "wechat-pen [flags] [file.md]",
	Short: "Markdown → 公众号可粘贴 HTML",
	Long: `wechat-pen converts Markdown to WeChat-editor-pasteable HTML with inline styles.

Reads from a file, or stdin when piped. Run "wechat-pen serve" for the web UI.`,
	RunE: runConvert,
	Args: cobra.MaximumNArgs(1),
}

var rootFlags convertFlags

func init() {
	rootFlags.register(rootCmd)
	rootCmd.AddCommand(serveCmd)
	rootCmd.AddCommand(batchCmd)
	rootCmd.AddCommand(watchCmd)
}

func runConvert(cmd *cobra.Command, args []string) error {
	input, err := resolveInput(args)
	if err != nil {
		return err
	}
	defer input.Close()

	src, err := io.ReadAll(input.Reader)
	if err != nil {
		return fmt.Errorf("read input: %w", err)
	}

	cfg := rootFlags.toConfig(input.Name)

	if rootFlags.css != "" {
		cssBytes, err := os.ReadFile(rootFlags.css)
		if err != nil {
			return fmt.Errorf("read css: %w", err)
		}
		cfg.CSS = string(cssBytes)
	}

	html, err := converter.Convert(src, cfg)
	if err != nil {
		return fmt.Errorf("convert: %w", err)
	}

	outPath := rootFlags.output
	if outPath == "" && input.Path != "" && input.Path != "-" {
		outPath = stripExt(input.Path) + ".html"
	}

	if outPath == "" {
		_, err = os.Stdout.Write(html)
		return err
	}

	if err := os.WriteFile(outPath, html, 0o644); err != nil {
		return fmt.Errorf("write output: %w", err)
	}
	fmt.Fprintf(os.Stderr, "wrote %s\n", outPath)
	return nil
}

// -- serve --

var serveFlags struct {
	addr      string
	themesDir string
	appID     string
	secret    string
}

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start web UI + API server",
	Example: `  wechat-pen serve
  wechat-pen serve --addr :3000
  wechat-pen serve --themes ./my-themes
  wechat-pen serve --appid wx123456 --secret abcdef`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if n, err := converter.LoadThemesDir(serveFlags.themesDir); err != nil {
			fmt.Fprintf(os.Stderr, "themes: %v\n", err)
		} else if n > 0 {
			fmt.Printf("themes: loaded %d pack(s) from %s\n", n, serveFlags.themesDir)
		}
		return server.ListenAndServe(serveFlags.addr, server.Options{
			ThemesDir: serveFlags.themesDir,
			AppID:     serveFlags.appID,
			Secret:    serveFlags.secret,
		})
	},
}

func init() {
	serveCmd.Flags().StringVar(&serveFlags.addr, "addr", ":8080", "listen address")
	serveCmd.Flags().StringVar(&serveFlags.themesDir, "themes", "themes", "external theme packs directory (*.json)")
	serveCmd.Flags().StringVar(&serveFlags.appID, "appid", "", "WeChat Official Account AppID")
	serveCmd.Flags().StringVar(&serveFlags.secret, "secret", "", "WeChat Official Account AppSecret")
}



// -- batch --

var batchFlags struct {
	dir       string
	outDir    string
	recursive bool
	convertFlags
}

var batchCmd = &cobra.Command{
	Use:   "batch",
	Short: "Convert all .md files in a directory",
	Example: `  wechat-pen batch
  wechat-pen batch --dir ./posts --out ./html --recursive
  wechat-pen batch --style tech --highlight`,
	RunE: runBatch,
}

func init() {
	batchFlags.register(batchCmd)
	batchCmd.Flags().StringVar(&batchFlags.dir, "dir", ".", "input directory")
	batchCmd.Flags().StringVar(&batchFlags.outDir, "out", "", "output directory (default: same as input)")
	batchCmd.Flags().BoolVarP(&batchFlags.recursive, "recursive", "r", false, "recurse into subdirectories")
}

func runBatch(cmd *cobra.Command, args []string) error {
	cfg := batchFlags.toConfig("")

	walkFn := func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path != batchFlags.dir && !batchFlags.recursive {
				return filepath.SkipDir
			}
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".md" && ext != ".markdown" {
			return nil
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		html, err := converter.Convert(src, cfg)
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		rel, _ := filepath.Rel(batchFlags.dir, path)
		targetDir := batchFlags.outDir
		if targetDir == "" {
			targetDir = filepath.Dir(path)
			rel = filepath.Base(path)
		}
		outPath := filepath.Join(targetDir, stripExt(rel)+".html")
		if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(outPath, html, 0o644); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "wrote %s\n", outPath)
		return nil
	}

	return filepath.WalkDir(batchFlags.dir, walkFn)
}

// -- watch --

var watchFlags struct {
	dir string
	convertFlags
}

var watchCmd = &cobra.Command{
	Use:   "watch",
	Short: "Watch directory and auto-convert on change",
	Example: `  wechat-pen watch
  wechat-pen watch --dir ./posts --style warm`,
	RunE: runWatch,
}

func init() {
	watchFlags.register(watchCmd)
	watchCmd.Flags().StringVar(&watchFlags.dir, "dir", ".", "directory to watch")
}

func runWatch(cmd *cobra.Command, args []string) error {
	cfg := watchFlags.toConfig("")
	fmt.Fprintf(os.Stderr, "watching %s (poll every 1s)… Ctrl+C to stop\n", watchFlags.dir)
	last := map[string]time.Time{}

	convertFile := func(path string) {
		src, err := os.ReadFile(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "read %s: %v\n", path, err)
			return
		}
		html, err := converter.Convert(src, cfg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "convert %s: %v\n", path, err)
			return
		}
		out := stripExt(path) + ".html"
		if err := os.WriteFile(out, html, 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "write %s: %v\n", out, err)
			return
		}
		fmt.Fprintf(os.Stderr, "wrote %s\n", out)
	}

	for {
		_ = filepath.WalkDir(watchFlags.dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			ext := strings.ToLower(filepath.Ext(path))
			if ext != ".md" && ext != ".markdown" {
				return nil
			}
			info, err := d.Info()
			if err != nil {
				return nil
			}
			mt := info.ModTime()
			if prev, ok := last[path]; !ok || mt.After(prev) {
				last[path] = mt
				if ok {
					convertFile(path)
				}
			}
			return nil
		})
		time.Sleep(time.Second)
	}
}

// -- helpers --

func stripExt(path string) string {
	ext := filepath.Ext(path)
	return strings.TrimSuffix(path, ext)
}

type inputSource struct {
	Reader io.ReadCloser
	Path   string
	Name   string
}

func (i inputSource) Close() error {
	if i.Reader != nil && i.Reader != os.Stdin {
		return i.Reader.Close()
	}
	return nil
}

func resolveInput(args []string) (inputSource, error) {
	if len(args) == 0 || args[0] == "-" {
		stat, err := os.Stdin.Stat()
		if err != nil {
			return inputSource{}, err
		}
		if (stat.Mode() & os.ModeCharDevice) != 0 {
			return inputSource{}, fmt.Errorf("no input: pass a .md file, pipe via stdin, or run: wechat-pen serve")
		}
		return inputSource{Reader: os.Stdin, Path: "-", Name: "stdin"}, nil
	}

	path := args[0]
	f, err := os.Open(path)
	if err != nil {
		return inputSource{}, fmt.Errorf("open input: %w", err)
	}
	base := filepath.Base(path)
	name := strings.TrimSuffix(base, filepath.Ext(base))
	return inputSource{Reader: f, Path: path, Name: name}, nil
}
