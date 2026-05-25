package api

import (
	gohttp "net/http"

	"github.com/gorilla/pat"
	"github.com/ian-kent/go-log/log"
	"github.com/mailhog/MailHog-Server/config"
	"github.com/mailhog/data"
)

const realtimeMessageBuffer = 1024

func CreateAPI(conf *config.Config, r gohttp.Handler) {
	apiv1 := createAPIv1(conf, r.(*pat.Router))
	apiv2 := createAPIv2(conf, r.(*pat.Router))
	apiv3 := createAPIv3(conf, r.(*pat.Router))

	go func() {
		for {
			select {
			case msg := <-conf.MessageChan:
				if apiv1.hasSubscribers() {
					enqueueRealtimeMessage("APIv1", apiv1.messageChan, msg)
				}
				if apiv2.hasSubscribers() {
					enqueueRealtimeMessage("APIv2", apiv2.messageChan, msg)
				}
				if apiv3.hasSubscribers() {
					enqueueRealtimeMessage("APIv3", apiv3.messageChan, msg)
				}
			}
		}
	}()
}

func newRealtimeMessageChan() chan *data.Message {
	return make(chan *data.Message, realtimeMessageBuffer)
}

func enqueueRealtimeMessage(name string, ch chan *data.Message, msg *data.Message) {
	if ch == nil || msg == nil {
		return
	}
	select {
	case ch <- msg:
	default:
		log.Printf("[%s] realtime queue full, dropping message event %s", name, msg.ID)
	}
}
