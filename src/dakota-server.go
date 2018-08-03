package main


import (
    "fmt"
    "log"
    "net/http"
    "strconv"
    "strings"
    "io"
    "io/ioutil"
    "os"
    "path/filepath"
    "bytes"    
    "os/signal"
    "context"
    "time"
    "html"    
    urls "net/url"
)

var PORT int = 8088
var AUTH_USER string = "admin"
var AUTH_PASS string = "adm1nF0rev@"

var USE_SSL bool  = false
var SSL_SERT_FILE = "ssl/cert.pem"
var SSL_KEY_FILE  = "ssl/key.pem"

var LOG_TO_FILE bool = true
var LOG_TO_FILE_NAME string = "../server.log"

var logFile *os.File
var SERVER_START_TIME time.Time = time.Now()



func main() {
    args := os.Args[1:]
    if (len(args) > 0 && args[0] == "stop") {
        os.Exit(stopServer());
    } else {
        startServer();
    }
}

func stopServer() int {
    setUpLog()
    
    log.Printf("Stopping server by url (port %d) ..", PORT);
    client := &http.Client{}
    
    // Check if server running
    req, err := http.NewRequest("GET", fmt.Sprintf("http://localhost:%d/static/notexist.ext", PORT), nil)
    req.SetBasicAuth(AUTH_USER, AUTH_PASS)
    aliveResp, err := client.Do(req)    
	if err != nil {
        log.Printf("ERROR while attempt to stop (it seems server is not running yet): %s", err.Error())
		return 0
	}
    defer aliveResp.Body.Close()
    
    // Ask to stop
    req, err = http.NewRequest("GET", fmt.Sprintf("http://localhost:%d/_stop/", PORT), nil)
    req.SetBasicAuth(AUTH_USER, AUTH_PASS)
    resp, err := client.Do(req)
    if err != nil {
		log.Printf("ERROR while attempt to stop: %s", err.Error())
        return 1
	}
    defer resp.Body.Close()
    
    time.Sleep(2000 * time.Millisecond)
    log.Printf("Server successfully stopped by url (port %d).", PORT);
    return 0
}

func startServer() {
    setUp()
    defer tearDown()
    startHttpServer(PORT, listenOSSignals())
}

// Return stop request channel
func listenOSSignals() chan int {
	var stopChannel chan int = make(chan int, 1)
    go func() {
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, os.Interrupt)
		<-sigint
        stopChannel <-1		
	}()
    return stopChannel
}


func setUp() {
    setUpLog()

    log.Printf("Server setUp\n")
    log.Printf("Exe dir: %q\n", getCurrentExeDir())
    log.Printf("USE_SSL: %t\n", USE_SSL)

    _ = os.Mkdir(getUploadsDir(), 0755)
}

func setUpLog() {
    if (LOG_TO_FILE) {
        logFile, err := os.OpenFile(getRelativeExePath(LOG_TO_FILE_NAME), os.O_RDWR | os.O_CREATE | os.O_APPEND, 0666)
        if err != nil {
            panic(err)
        }
        log.SetOutput(logFile)
    }
}

func getUploadsDir() string {
    return getRelativeExePath("../uploads");
}

func tearDown() {
    if (logFile != nil) {
        logFile.Close()
        logFile = nil;
    }
}


func startHttpServer(port int, stopCh chan int) {
    srv := &http.Server{Addr: ":" + strconv.Itoa(port)}

    http.HandleFunc("/", authDecorator(handler))
    http.HandleFunc("/uploads/", statsDecorator(handlerStatic))
    http.HandleFunc("/upload/", authDecorator(handlerUpload))
    http.HandleFunc("/delete/", statsDecorator(authDecorator(handlerDelete)))
    http.HandleFunc("/static/", statsDecorator(authDecorator(handlerStatic)))
    
    http.HandleFunc("/_stop/", statsDecorator(authDecorator(makeHaldlerStop(stopCh))))

    log.Printf("Server started on port %d\n", port)

    go func() {
        var err error        
        if (USE_SSL) {
            err = srv.ListenAndServeTLS(getRelativeExePath(SSL_SERT_FILE), getRelativeExePath(SSL_KEY_FILE))
        } else {
            err = srv.ListenAndServe();
        }
        
        if err != nil && err != http.ErrServerClosed {
            log.Printf("Server stoped with error: %s", err)
        }
    }()
    
    // Wait stop request
    <-stopCh
    
    log.Println("Got stop request, stopping..")
    ctx, _ := context.WithTimeout(context.Background(), 3*time.Second)
    srv.Shutdown(ctx)
    log.Println("Server stopped")
}


func makeHaldlerStop(stopCh chan int) func(http.ResponseWriter, *http.Request) {
    return func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "text/plain; charset=utf-8")
        fmt.Fprintf(w, "Good bye!")
        stopCh<- 1
    }
}


func handler(w http.ResponseWriter, r *http.Request) {    
    path := r.URL.Path;
    if (path == "/favicon.ico") {
        log.Printf("Default routing request (favicon). returned 404\n")
        http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound) 
    } else {
        w.Header().Set("Content-Type", "text/html; charset=utf-8")
        content := []byte(getIndexHtml());
        count := 0
        var err error
        for count < len(content) {
            count, err = w.Write(content[count:])
            if (err != nil) {
                log.Printf("ERROR: failed to write file to response.\nError: %s", path, err.Error())
                return
            }
        }
                
        //http.Redirect(w, r, "/static/index.html", 302)
    }
}


func handlerStatic(w http.ResponseWriter, r *http.Request) {
    path := r.URL.Path;
    
    if (!strings.Contains(path, "..")) {
        filename := getRelativeExePath(path)
        if (strings.HasPrefix(path, "/uploads/")) {
            filename = filepath.Join(getUploadsDir(), strings.TrimPrefix(path, "/uploads"))
        }

        // TODO: Жрет память! Либо делать стрим из файла либо настроить nginx на перехват /uploads/ 
        // (натравил nginx)
        content, err := ioutil.ReadFile(filename); 
        if (err != nil) {
            log.Printf("Warning: not found file: %q (%q)\n  I/O error: %s", 
                path, filename, err.Error())
            http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
            return
        }        
        
        w.Header().Set("Content-Type", getContentType(filename))
        
        count := 0
        for count < len(content) {
            count, err = w.Write(content[count:])
            if (err != nil) {
                log.Printf("ERROR: failed to write file to response.\nError: %s", path, err.Error())
                return
            }
        }
        
    } else {        
        log.Printf("Warning: attempt to access out of static folders. Path: %q", path)
        http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
    }    
}


func handlerDelete(w http.ResponseWriter, r *http.Request) {
    path := r.URL.Path;
    if (!strings.Contains(path, "..")) {
        fileName := path
        if (strings.HasPrefix(path, "/delete/")) {
            fileName = filepath.Join(getUploadsDir(), strings.TrimPrefix(path, "/delete"))
        }

        
        var err = os.Remove(fileName)
        if (err != nil) {
            log.Printf("ERROR deleting file: %q\n%v", fileName, err)
        }
        
        http.Redirect(w, r, "/", 302)
    } else {        
        log.Printf("Warning: attempt to access out of static folders. Path: %q", path)
        http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
    } 
}


func readFile(filename string) ([]byte, error) {
    return ioutil.ReadFile(filename)
}


func getContentType(filePath string) string {
    if strings.HasSuffix(filePath, ".html") || strings.HasSuffix(filePath, ".htm") {
        return "text/html; charset=utf-8"
    }
    if strings.HasSuffix(filePath, ".txt") {
        return "text/plain; charset=utf-8"
    }
    if strings.HasSuffix(filePath, ".jpg") {
        return "image/jpeg"
    }
    if strings.HasSuffix(filePath, ".pdf") {
        return "application/pdf"
    }    

    return "application/octet-stream"
}


func getCurrentExeDir() string {
    dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
    if err != nil {
        log.Fatal(err)
    }
    return dir
}


func handlerUpload(w http.ResponseWriter, r *http.Request) {
    if r.Method == "POST" {
        r.ParseMultipartForm(32 << 20)
        file, handler, err := r.FormFile("uploadfile")
        if err != nil {
            log.Printf("Upload error: %s\n", err)
            return
        }
        defer file.Close()        
        f, err := os.OpenFile(getUploadsDir() + "/" + handler.Filename, os.O_WRONLY|os.O_CREATE, 0666)
        if err != nil {
            log.Printf("ERROR save uploading file: %s\n", err)
            fmt.Fprintf(w, "Error saving file")
            return
        }
        
        defer f.Close()
        size, err := io.Copy(f, file)
        if (err != nil) {
            log.Printf("Uploaded filed: %s\n", err.Error())
        }
                
        http.Redirect(w, r, "/", 301)
        log.Printf("ip: %s, uploaded file: %s, size: %d\n", r.RemoteAddr, handler.Filename, int(size))
    } else {
        http.Error(w, "Expected method: POST", http.StatusNotFound)
    }
}


func handlerList(w http.ResponseWriter, r *http.Request) {
    var buffer bytes.Buffer
    
    list, err := getUploadedFileList()
    if (err != nil) {
        fmt.Fprintf(w, "Error: %s", err)
        return
    }
    
    buffer.WriteString("<html><body>")
        
    renderFileListAsTable(&buffer, list)
    
    buffer.WriteString("<br/><a href=\"/\">Home</a>")
    buffer.WriteString("</body></html>")
    
    w.Header().Set("Content-Type", "text/html; charset=utf-8")
    
    fmt.Fprintf(w, "%s", buffer.String())     
}


func getIndexHtml() string {
    var buffer bytes.Buffer
    
    list, err := getUploadedFileList()
    if (err != nil) {        
        return err.Error()
    }
    
    buffer.WriteString(`<html><head><meta charset="UTF-8"></head><body><h2>Upload page</h2>`)
    buffer.WriteString(`<form enctype="multipart/form-data" action="/upload/" method="post">
        <input type="file" name="uploadfile" />
        <input type="submit" value="upload" />
        </form>`)    
        
    renderFileListAsTable(&buffer, list)
    
    buffer.WriteString("<br/><hr/><small>")
    buffer.WriteString("Server uptime: " + getUptime() + "<br/>")
    buffer.WriteString("Git SHA: " + getGitSha())
    buffer.WriteString("</small>")
    
    buffer.WriteString("</body></html>")
    return buffer.String()
}

func getUptime() string {
    uptime  := time.Since(SERVER_START_TIME);
    secs    := int64(uptime.Seconds())
    minutes := int(secs / 60)
    hours   := int(minutes / 60)
    days    := int(hours / 24)    
    
    uptimeStr := fmt.Sprintf("%d day(s), %02d:%02d:%02d",
        days, int(hours % 24), int(minutes % 60), int(secs % 60))
        
    return uptimeStr
}

func getGitSha() string {
    filename := getRelativeExePath("/static/git_sha.txt")
    content, err := ioutil.ReadFile(filename);
    if (err != nil) {
        return ""
    }        
    return string(content);
}

func renderFileListAsTable(buffer *bytes.Buffer, files []os.FileInfo) {
    buffer.WriteString("<table>")
    buffer.WriteString("<thead>")
    buffer.WriteString("<tr>")
    buffer.WriteString("<td>Name</td>")
    buffer.WriteString("<td>Size</td>")
    buffer.WriteString("<td>Time</td>")
    buffer.WriteString("<td></td>")
    buffer.WriteString("</tr>")
    buffer.WriteString("</thead>")
    
    for _, f := range files {        
        url := "/uploads/" + urls.PathEscape(f.Name()); 
        delUrl := "/delete/" + urls.PathEscape(f.Name());
        
        buffer.WriteString("<tr>")
        buffer.WriteString("<td>" + fmt.Sprintf("<a href=\"%s\">%s</a>", url, html.EscapeString(f.Name())) + "</td>")
        buffer.WriteString("<td>" + fmt.Sprintf("%d", f.Size()) + "</td>")
        buffer.WriteString("<td>" + fmt.Sprintf("%s", f.ModTime().String()) + "</td>")
        buffer.WriteString("<td>" + fmt.Sprintf("<a href=\"%s\">delete</a>", delUrl) + "</td>")
        
        buffer.WriteString("</tr>")
    }
    buffer.WriteString("</table>")
}


func getUploadedFileList() ([]os.FileInfo, error) {
    files, err := ioutil.ReadDir(getUploadsDir())
    if err != nil {
        log.Printf("Error reading uploaded file list: %s", err)
        return files, err
    }

    var result []os.FileInfo = make([]os.FileInfo, 0)
    
    for _, f := range files {
        if (!f.IsDir()) {
            result = append(result, f)
        }
    }
    
    return result, nil
}


func getRelativeExePath(path string) string {
    return filepath.Join(getCurrentExeDir(), path)
}


func authDecorator(fn http.HandlerFunc) http.HandlerFunc {
  return func(w http.ResponseWriter, r *http.Request) {
    user, pass, _ := r.BasicAuth()
    if !checkAuth(user, pass) {
        w.Header().Set("WWW-Authenticate", "Basic realm=\"MY REALM\"")
        http.Error(w, "Unauthorized.", 401)
        return
    }
    fn(w, r)
  }
}


func checkAuth(user, pass string) bool {
    return (AUTH_USER == "" && AUTH_PASS == "") || (user == AUTH_USER && pass == AUTH_PASS)
}


func statsDecorator(fn http.HandlerFunc) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {            
        ip := r.RemoteAddr
        uri := r.RequestURI
        log.Printf("ip: %s, uri: %s\n", ip, uri)
        
        fn(w, r)
    }
}
