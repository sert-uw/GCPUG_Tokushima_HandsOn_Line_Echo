package app

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/url"
	"os"

	"google.golang.org/appengine"
	"google.golang.org/appengine/log"
	"google.golang.org/appengine/taskqueue"
	"google.golang.org/appengine/urlfetch"

	"golang.org/x/net/context"

	"github.com/joho/godotenv"
	"github.com/line/line-bot-sdk-go/linebot"
	"github.com/line/line-bot-sdk-go/linebot/httphandler"
)

var botHandler *httphandler.WebhookHandler

// 初期化処理
func init() {
	// line.envの読み込み
	err := godotenv.Load("line.env")
	if err != nil {
		panic(err)
	}

	// lineのhttphandlerを設定
	botHandler, err = httphandler.New(
		os.Getenv("LINE_BOT_CHANNEL_SECRET"),
		os.Getenv("LINE_BOT_CHANNEL_TOKEN"),
	)
	botHandler.HandleEvents(handleCallback)

	http.Handle("/callback", botHandler)
	http.HandleFunc("/task", handleTask)
}

// webhook の受付関数
func handleCallback(evs []*linebot.Event, r *http.Request) {
	c := newContext(r)
	ts := make([]*taskqueue.Task, len(evs))
	for i, e := range evs {
		j, err := json.Marshal(e)
		if err != nil {
			log.Errorf(c, "json.Marshal: %v", err)
			return
		}
		data := base64.StdEncoding.EncodeToString(j)
		t := taskqueue.NewPOSTTask("/task", url.Values{"data": {data}})
		ts[i] = t
	}
	taskqueue.AddMulti(c, ts, "")
}

// 受信したメッセージへの返信処理
func handleTask(w http.ResponseWriter, r *http.Request) {
	c := newContext(r)
	data := r.FormValue("data")

	if data == "" {
		log.Errorf(c, "No data")
		return
	}

	// メッセージJSONのパース
	j, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		log.Errorf(c, "base64 DecodeString: %v", err)
		return
	}

	e := new(linebot.Event)
	err = json.Unmarshal(j, e)
	if err != nil {
		log.Errorf(c, "json.Unmarshal: %v", err)
		return
	}

	// LINE bot 変数の生成
	bot, err := newLINEBot(c)
	if err != nil {
		log.Errorf(c, "newLINEBot: %v", err)
		return
	}

	log.Infof(c, "EventType: %s\nMessage: %#v", e.Type, e.Message)
	var responseMessage linebot.Message

	// 受信したメッセージのタイプチェック
	switch message := e.Message.(type) {
	// テキストメッセージの場合
	case *linebot.TextMessage:
		responseMessage = linebot.NewTextMessage(message.Text)
	default:
		responseMessage = linebot.NewTextMessage("未対応です。。。")
	}

	// 生成したメッセージを送信する
	if _, err = bot.ReplyMessage(e.ReplyToken, responseMessage).WithContext(c).Do(); err != nil {
		log.Errorf(c, "ReplayMessage: %v", err)
		return
	}

	w.WriteHeader(200)
}

func newContext(r *http.Request) context.Context {
	return appengine.NewContext(r)
}

func newLINEBot(c context.Context) (*linebot.Client, error) {
	return botHandler.NewClient(
		linebot.WithHTTPClient(urlfetch.Client(c)),
	)
}
