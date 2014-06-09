package main

import (
  "flag"
  "fmt"
  "github.com/russross/blackfriday"
  "html"
  "io/ioutil"
  "log"
  "net/http"
  "os"
  "path/filepath"
  "strings"
)

type (
  options struct {
    root       string
    port       uint
    recursive  bool
    stylesheet string
    extension  string
  }
)

func main() {
  opts := parseArguments()

  log.Printf("Starting server at port (%d) and root (%s)\n", opts.port, opts.root)
  log.Printf("Press CTRL-C to terminate\n")

  http.HandleFunc("/", handle(opts))
  error := http.ListenAndServe(fmt.Sprintf(":%d", opts.port), nil)
  if error != nil {
    log.Fatal("Error: could not listen at ", opts.port)
  }
}

// Read and validate the command line arguments.
func parseArguments() options {
  var opts options

  flag.StringVar(&opts.root, "root", ".", "root folder containing the Markdown documents")
  flag.UintVar(&opts.port, "port", 8888, "port the server should use")
  flag.BoolVar(&opts.recursive, "recursive", true, "allow serving Markdown documents within the subdirectories of the root")
  flag.StringVar(&opts.stylesheet, "stylesheet", staticStylesheetName(), "stylesheet to use when rendering Markdown files")
  flag.StringVar(&opts.extension, "extension", ".md", "extension identifying the Markdown files")
  flag.Usage = func() {
    fmt.Fprintln(os.Stderr, "MarkUp: a tiny Markdown server")
    fmt.Fprintln(os.Stderr, "Usage:")
    flag.PrintDefaults()
  }

  flag.Parse()

  root, error := filepath.Abs(opts.root)
  if error == nil {
    opts.root = root
  } else {
    log.Fatalf("Error: could not find the directory: %s", opts.root)
  }
  stat, error := os.Stat(opts.root)
  if error != nil || !stat.IsDir() {
    log.Fatalf("Error: could not find the directory: %s", opts.root)
  }

  if opts.port > 65535 {
    log.Fatalf("Error: the port number (%d) is not number", opts.port)
  }

  if opts.stylesheet != staticStylesheetName() && !strings.HasPrefix(opts.stylesheet, "http://") {
    log.Fatalf("Error: the stylesheet (\"%s\") should be a URL starting with http", opts.stylesheet)
  }

  if !strings.HasPrefix(opts.extension, ".") {
    log.Fatalf("Error: the extension (\"%s\") should start with a dot", opts.extension)
  }
  return opts
}

// Handle all incoming requests.
func handle(opts options) func(http.ResponseWriter, *http.Request) {
  return func(w http.ResponseWriter, r *http.Request) {
    if !handleStaticResource(w, r) {
      urlPath := r.URL.Path
      path := filepath.FromSlash(filepath.Clean(opts.root + "/" + urlPath))

      stat, error := os.Stat(path)
      if error != nil {
        handleError(w, urlPath)
      } else if stat.IsDir() {
        handleDir(w, path, urlPath, opts)
      } else {
        handleFile(w, path, urlPath, opts)
      }
    }
  }
}

// Handle requests for directories.
//
// Directories are rendered by generating relative links for the top-level
// Markdown files and subdirectories within it.
// The filtering of Markdown files is solely based on the files' extensions
// without introspecting their content.
func handleDir(w http.ResponseWriter, path string, urlPath string, opts options) {
  addLink := func(w http.ResponseWriter, url string, label string) {
    fmt.Fprintf(w, "<a href=\"%s\"><tt>%s</tt></a><br>", url, label)
  }
  addRelativeLink := func(w http.ResponseWriter, path string, name string) {
    addLink(w, strings.TrimPrefix(path, opts.root), name)
  }
  walker := func(file string, finfo os.FileInfo, error error) error {
    if error != nil {
      handleError(w, urlPath)
      return error
    }
    if finfo.IsDir() && file != path {
      if opts.recursive && !strings.HasPrefix(finfo.Name(), ".") { // ignore dot files
        addRelativeLink(w, file, finfo.Name()+"/") // signal directories with a trailing slash
      }
      return filepath.SkipDir // add links only for the files within the current directory
    }
    if strings.EqualFold(filepath.Ext(file), opts.extension) { // add links only for the Markdown files
      addRelativeLink(w, file, finfo.Name())
    }
    return nil
  }

  writePageStart(w, urlPath)
  filepath.Walk(path, walker)
  writePageEnd(w)
}

// Handle requests for Markdown files.
//
// Currently markdown files are read and rendered for each request. The rendered
// Markdown is not cached on disk or in memory server-side (beyond the Operating
// System's file cache). Additionally, browser-side caching is not enabled
// (for example using ETags based on the file's modified date).
func handleFile(w http.ResponseWriter, path string, urlPath string, opts options) {
  // ignore any file without the Markdown extension
  if !strings.EqualFold(filepath.Ext(path), opts.extension) {
    handleError(w, urlPath)
    return
  }

  file, error := os.Open(path)
  if error != nil {
    handleError(w, urlPath)
    return
  }
  defer file.Close()

  content, error := ioutil.ReadAll(file)
  if error != nil {
    handleError(w, urlPath)
    return
  }

  var rendererOptions = blackfriday.HTML_COMPLETE_PAGE|blackfriday.HTML_USE_SMARTYPANTS
  var renderer = blackfriday.HtmlRenderer(rendererOptions, urlPath, opts.stylesheet)
  var enabledExtensions = blackfriday.EXTENSION_AUTOLINK|blackfriday.EXTENSION_FENCED_CODE|blackfriday.EXTENSION_STRIKETHROUGH
  w.Write(blackfriday.Markdown(content, renderer, enabledExtensions))
}

// Handle read errors.
//
// For security reasons, the same message is returned irrespective of the
// actual read error (e.g. the file does not exist, the file path points to
// a directory, the file is not a Markdown file or the current user does
// not have read permissions on the file)
func handleError(w http.ResponseWriter, urlPath string) {
  w.WriteHeader(http.StatusNotFound)
  writePageStart(w, "File not found")
  fmt.Fprintf(w, "File not found: %s", urlPath)
  writePageEnd(w)
}

// Handles requests for the built-in static resource(s).
//
// Currently the only built-in static resource is
// a CSS stylesheet to style the rendered Markdown.
func handleStaticResource(w http.ResponseWriter, r *http.Request) bool {
  if r.URL.Path == staticStylesheetName() {
    writeHeaders(w, "text/css")
    fmt.Fprintf(w, staticStylesheet())
    return true
  }

  return false
}

func writeHeaders(w http.ResponseWriter, contentType string) {
  w.Header().Set("Content-Type", contentType)
}

func writePageStart(w http.ResponseWriter, title string) {
  writeHeaders(w, "text/html")
  fmt.Fprintf(w, "<!DOCTYPE html><html><head><title>%s</title></head><body>", html.EscapeString(title))
}

func writePageEnd(w http.ResponseWriter) {
  fmt.Fprintf(w, "</body></html>")
}

// ---- built-in static resources ----

func staticStylesheetName() string {
  return "/static/stylesheet.css"
}

// Built-in stylesheet that mimicks the default Github Markdown CSS stylesheet.
//
// Credits for the stylesheet goes to Andy Ferra (https://github.com/andyferra):
// https://gist.github.com/2554919/10ce87fe71b23216e3075d5648b8b9e56f7758e1
func staticStylesheet() string {
  return `
body {
  font-family: Helvetica, arial, sans-serif;
  font-size: 14px;
  line-height: 1.6;
  padding-top: 10px;
  padding-bottom: 10px;
  background-color: white;
  padding: 30px; }

body > *:first-child {
  margin-top: 0 !important; }
body > *:last-child {
  margin-bottom: 0 !important; }

a {
  color: #4183C4; }
a.absent {
  color: #cc0000; }
a.anchor {
  display: block;
  padding-left: 30px;
  margin-left: -30px;
  cursor: pointer;
  position: absolute;
  top: 0;
  left: 0;
  bottom: 0; }

h1, h2, h3, h4, h5, h6 {
  margin: 20px 0 10px;
  padding: 0;
  font-weight: bold;
  -webkit-font-smoothing: antialiased;
  cursor: text;
  position: relative; }

h1:hover a.anchor, h2:hover a.anchor, h3:hover a.anchor, h4:hover a.anchor, h5:hover a.anchor, h6:hover a.anchor {
  background: url("../../images/modules/styleguide/para.png") no-repeat 10px center;
  text-decoration: none; }

h1 tt, h1 code {
  font-size: inherit; }

h2 tt, h2 code {
  font-size: inherit; }

h3 tt, h3 code {
  font-size: inherit; }

h4 tt, h4 code {
  font-size: inherit; }

h5 tt, h5 code {
  font-size: inherit; }

h6 tt, h6 code {
  font-size: inherit; }

h1 {
  font-size: 28px;
  color: black; }

h2 {
  font-size: 24px;
  border-bottom: 1px solid #cccccc;
  color: black; }

h3 {
  font-size: 18px; }

h4 {
  font-size: 16px; }

h5 {
  font-size: 14px; }

h6 {
  color: #777777;
  font-size: 14px; }

p, blockquote, ul, ol, dl, li, table, pre {
  margin: 15px 0; }

hr {
  background: transparent url("../../images/modules/pulls/dirty-shade.png") repeat-x 0 0;
  border: 0 none;
  color: #cccccc;
  height: 4px;
  padding: 0; }

body > h2:first-child {
  margin-top: 0;
  padding-top: 0; }
body > h1:first-child {
  margin-top: 0;
  padding-top: 0; }
  body > h1:first-child + h2 {
    margin-top: 0;
    padding-top: 0; }
body > h3:first-child, body > h4:first-child, body > h5:first-child, body > h6:first-child {
  margin-top: 0;
  padding-top: 0; }

a:first-child h1, a:first-child h2, a:first-child h3, a:first-child h4, a:first-child h5, a:first-child h6 {
  margin-top: 0;
  padding-top: 0; }

h1 p, h2 p, h3 p, h4 p, h5 p, h6 p {
  margin-top: 0; }

li p.first {
  display: inline-block; }

ul, ol {
  padding-left: 30px; }

ul :first-child, ol :first-child {
  margin-top: 0; }

ul :last-child, ol :last-child {
  margin-bottom: 0; }

dl {
  padding: 0; }
  dl dt {
    font-size: 14px;
    font-weight: bold;
    font-style: italic;
    padding: 0;
    margin: 15px 0 5px; }
    dl dt:first-child {
      padding: 0; }
    dl dt > :first-child {
      margin-top: 0; }
    dl dt > :last-child {
      margin-bottom: 0; }
  dl dd {
    margin: 0 0 15px;
    padding: 0 15px; }
    dl dd > :first-child {
      margin-top: 0; }
    dl dd > :last-child {
      margin-bottom: 0; }

blockquote {
  border-left: 4px solid #dddddd;
  padding: 0 15px;
  color: #777777; }
  blockquote > :first-child {
    margin-top: 0; }
  blockquote > :last-child {
    margin-bottom: 0; }

table {
  padding: 0; }
  table tr {
    border-top: 1px solid #cccccc;
    background-color: white;
    margin: 0;
    padding: 0; }
    table tr:nth-child(2n) {
      background-color: #f8f8f8; }
    table tr th {
      font-weight: bold;
      border: 1px solid #cccccc;
      text-align: left;
      margin: 0;
      padding: 6px 13px; }
    table tr td {
      border: 1px solid #cccccc;
      text-align: left;
      margin: 0;
      padding: 6px 13px; }
    table tr th :first-child, table tr td :first-child {
      margin-top: 0; }
    table tr th :last-child, table tr td :last-child {
      margin-bottom: 0; }

img {
  max-width: 100%; }

span.frame {
  display: block;
  overflow: hidden; }
  span.frame > span {
    border: 1px solid #dddddd;
    display: block;
    float: left;
    overflow: hidden;
    margin: 13px 0 0;
    padding: 7px;
    width: auto; }
  span.frame span img {
    display: block;
    float: left; }
  span.frame span span {
    clear: both;
    color: #333333;
    display: block;
    padding: 5px 0 0; }
span.align-center {
  display: block;
  overflow: hidden;
  clear: both; }
  span.align-center > span {
    display: block;
    overflow: hidden;
    margin: 13px auto 0;
    text-align: center; }
  span.align-center span img {
    margin: 0 auto;
    text-align: center; }
span.align-right {
  display: block;
  overflow: hidden;
  clear: both; }
  span.align-right > span {
    display: block;
    overflow: hidden;
    margin: 13px 0 0;
    text-align: right; }
  span.align-right span img {
    margin: 0;
    text-align: right; }
span.float-left {
  display: block;
  margin-right: 13px;
  overflow: hidden;
  float: left; }
  span.float-left span {
    margin: 13px 0 0; }
span.float-right {
  display: block;
  margin-left: 13px;
  overflow: hidden;
  float: right; }
  span.float-right > span {
    display: block;
    overflow: hidden;
    margin: 13px auto 0;
    text-align: right; }

code, tt {
  margin: 0 2px;
  padding: 0 5px;
  white-space: nowrap;
  border: 1px solid #eaeaea;
  background-color: #f8f8f8;
  border-radius: 3px; }

pre code {
  margin: 0;
  padding: 0;
  white-space: pre;
  border: none;
  background: transparent; }

.highlight pre {
  background-color: #f8f8f8;
  border: 1px solid #cccccc;
  font-size: 13px;
  line-height: 19px;
  overflow: auto;
  padding: 6px 10px;
  border-radius: 3px; }

pre {
  background-color: #f8f8f8;
  border: 1px solid #cccccc;
  font-size: 13px;
  line-height: 19px;
  overflow: auto;
  padding: 6px 10px;
  border-radius: 3px; }
  pre code, pre tt {
    background-color: transparent;
    border: none; }
  `

}
