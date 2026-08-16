package pikapika

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"

	source "github.com/niuhuan/pica-go"
	"pikapika/pikapika/database/comic_center"
)

// ---------- 找回密码 ----------

func forgotPassword(email string) (string, error) {
	result, err := client.ForgotPassword(email)
	if err != nil {
		return "", err
	}
	return serialize(result, nil)
}

func resetPassword(params string) (string, error) {
	var paramsStruct struct {
		Email      string `json:"email"`
		QuestionNo int    `json:"questionNo"`
		Answer     string `json:"answer"`
	}
	err := json.Unmarshal([]byte(params), &paramsStruct)
	if err != nil {
		return "", err
	}
	result, err := client.ResetPassword(paramsStruct.Email, paramsStruct.QuestionNo, paramsStruct.Answer)
	if err != nil {
		return "", err
	}
	return serialize(result, nil)
}

// ---------- 导入其他程序的历史记录 ----------

// importComicViewFormOff 从拷贝来的 comic_center.db 导入浏览记录
func importComicViewFormOff(dbPath string) error {
	views, err := comic_center.ReadViewLogsFromFile(dbPath)
	if err != nil {
		return err
	}
	for i := range views {
		view := &views[i]
		err := comic_center.MergeViewLog(view)
		if err != nil {
			return err
		}
	}
	return nil
}

// ---------- Web 服务器 (局域网浏览器看下载的漫画) ----------

var webServerMutex sync.Mutex
var webServer *http.Server
var webServerListener net.Listener

func startWebServer() error {
	webServerMutex.Lock()
	defer webServerMutex.Unlock()
	if webServer != nil {
		return errors.New("web server already running")
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", webHandleIndex)
	mux.HandleFunc("/api/comics", webHandleComics)
	mux.HandleFunc("/api/comics/", webHandleComicResource)
	ln, err := net.Listen("tcp", "0.0.0.0:8080")
	if err != nil {
		return err
	}
	webServerListener = ln
	webServer = &http.Server{Handler: mux}
	go webServer.Serve(ln)
	return nil
}

func stopWebServer() error {
	webServerMutex.Lock()
	defer webServerMutex.Unlock()
	if webServer != nil {
		webServer.Close()
		webServer = nil
		if webServerListener != nil {
			webServerListener.Close()
			webServerListener = nil
		}
	}
	return nil
}

// 首页: 漫画列表页
func webHandleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(webIndexHtml))
}

// GET /api/comics 全部下载
func webHandleComics(w http.ResponseWriter, r *http.Request) {
	downloads, err := comic_center.AllDownloads()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(downloads)
}

// GET /api/comics/{id}        漫画详情(含章节)
// GET /api/comics/{id}/eps/{epOrder}/pictures/{rank}  图片
// GET /api/comics/{id}/logo    封面
func webHandleComicResource(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/comics/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		http.Error(w, "not found", 404)
		return
	}
	comicId := parts[0]
	if len(parts) == 1 {
		// 漫画详情 + 章节 + 图片信息
		comic, err := comic_center.FindComicDownloadById(comicId)
		if err != nil || comic == nil {
			http.Error(w, "not found", 404)
			return
		}
		eps, err := comic_center.ListDownloadEpByComicId(comicId)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		type webEp struct {
			comic_center.ComicDownloadEp
			Pictures []comic_center.ComicDownloadPicture `json:"pictures"`
		}
		type webComic struct {
			comic_center.ComicDownload
			Eps []webEp `json:"eps"`
		}
		wc := webComic{ComicDownload: *comic}
		for _, ep := range eps {
			pictures, _ := comic_center.ListDownloadPictureByEpId(ep.ID)
			wc.Eps = append(wc.Eps, webEp{ComicDownloadEp: ep, Pictures: pictures})
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(wc)
		return
	}
	if parts[1] == "logo" {
		comic, err := comic_center.FindComicDownloadById(comicId)
		if err != nil || comic == nil || comic.ThumbLocalPath == "" {
			http.Error(w, "not found", 404)
			return
		}
		buff, err := os.ReadFile(downloadPath(comic.ThumbLocalPath))
		if err != nil {
			http.Error(w, err.Error(), 404)
			return
		}
		w.Header().Set("Content-Type", "image")
		w.Write(buff)
		return
	}
	// /eps/{epOrder}/pictures/{rank}
	if len(parts) >= 6 && parts[1] == "eps" {
		epOrder, err1 := strconv.Atoi(parts[2])
		rank, err2 := strconv.Atoi(parts[5])
		if err1 != nil || err2 != nil || parts[3] != "pictures" {
			http.Error(w, "bad request", 400)
			return
		}
		picture, err := comic_center.FindDownloadPictureByOrder(comicId, int32(epOrder), int32(rank))
		if err != nil || picture == nil {
			http.Error(w, "not found", 404)
			return
		}
		buff, err := os.ReadFile(downloadPath(picture.LocalPath))
		if err != nil {
			http.Error(w, err.Error(), 404)
			return
		}
		w.Header().Set("Content-Type", "image/"+picture.Format)
		w.Write(buff)
		return
	}
	http.NotFound(w, r)
}

var _ = source.ComicInfo{}
var _ = fmt.Sprintf

const webIndexHtml = `<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>Pikapika 下载</title>
<style>
body{font-family:sans-serif;margin:0;background:#f5f5f5}
.card{background:#fff;margin:10px;padding:12px;border-radius:8px;box-shadow:0 1px 3px rgba(0,0,0,.12);cursor:pointer;display:flex;align-items:center}
.card img{width:64px;height:64px;object-fit:cover;margin-right:12px;border-radius:4px}
.card .t{font-weight:bold;font-size:16px}
.card .s{color:#888;font-size:13px;margin-top:4px}
#detail{padding:12px}
#detail img{max-width:100%;display:block;margin:6px 0}
.ep{background:#fff;margin:6px 0;padding:10px;border-radius:6px;cursor:pointer}
body.dark{background:#222}
body.dark .card,body.dark .ep{background:#333;color:#eee}
body.dark .card .s{color:#aaa}
</style>
</head>
<body>
<div id="app"></div>
<script>
let comics=[];
async function loadComics(){
  const r=await fetch('/api/comics');
  comics=await r.json();
  const app=document.getElementById('app');
  app.innerHTML='';
  comics.forEach(c=>{
    const d=document.createElement('div');
    d.className='card';
    d.onclick=()=>openComic(c.id);
    d.innerHTML='<img src="/api/comics/'+c.id+'/logo"><div><div class="t">'+c.title+'</div><div class="s">'+c.author+' · '+(c.selectedEpCount+"话")+'</div></div>';
    app.appendChild(d);
  });
}
async function openComic(id){
  const r=await fetch('/api/comics/'+id);
  const c=await r.json();
  const app=document.getElementById('app');
  app.innerHTML='<div id="detail"><h2>'+c.title+'</h2><button onclick="loadComics()">返回</button><div id="eps"></div></div>';
  const eps=document.getElementById('eps');
  c.eps.forEach(ep=>{
    const e=document.createElement('div');
    e.className='ep';
    e.innerHTML='<b>'+ep.title+'</b> ('+ep.pictures.length+'P)';
    e.onclick=()=>openEp(c,ep);
    eps.appendChild(e);
  });
}
function openEp(c,ep){
  const app=document.getElementById('app');
  app.innerHTML='<div id="detail"><h2>'+c.title+' - '+ep.title+'</h2><button onclick="openComic(\''+c.id+'\')">返回</button><div id="pics"></div></div>';
  const pics=document.getElementById('pics');
  ep.pictures.forEach(p=>{
    const img=document.createElement('img');
    img.src='/api/comics/'+c.id+'/eps/'+ep.epOrder+'/pictures/'+p.rankInEp;
    pics.appendChild(img);
  });
}
loadComics();
</script>
</body>
</html>`
