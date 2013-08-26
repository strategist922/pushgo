package router

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mozilla.org/util"
	"net"
	"sync"
)

type Router struct {
	Port   string
	Logger *util.HekaLogger
}

type Route struct {
	socket net.Conn
}

var MuRoutes sync.Mutex
var routes map[string]*Route

type Update struct {
	Uaid string `json:"uaid"`
	Chid string `json:"chid"`
	Vers int64  `json:"vers"`
}

type Updater func(*Update) error

func (self *Router) HandleUpdates(updater Updater) {
	listener, err := net.Listen("tcp", ":"+self.Port)
	if err != nil {
		if self.Logger != nil {
			self.Logger.Critical("router",
				"Could not open listener:"+err.Error(), nil)
		} else {
			log.Printf("error listening %s", err.Error())
		}
		return
	}
	log.Printf("Listening for updates on *:" + self.Port)

	for {
		conn, err := listener.Accept()
		if err != nil {
			if self.Logger != nil {
				self.Logger.Critical("router",
					"Could not accept connection:"+err.Error(), nil)
			} else {
				log.Printf("Could not accept listener:%s", err.Error())
			}
		}
		go self.doupdate(updater, conn)
	}
}

func (self *Router) doupdate(updater Updater, conn net.Conn) (err error) {
	buf := make([]byte, 1024)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			if err == io.EOF && n == 0 {
				if self.Logger != nil {
					self.Logger.Debug("router", "@@@@ Closing listener socket."+err.Error(), nil)
				}
				err = nil
				break
			}
			break
		}
		log.Printf("@@@ Updates::: " + string(buf[:n]))
		update := Update{}
		items := bytes.Split(buf[:n], []byte("\n"))
		for _, item := range items {
			log.Printf("@@@ item ::: %s", item)
			if len(item) == 0 {
				continue
			}
			json.Unmarshal(item, &update)
			if self.Logger != nil {
				self.Logger.Debug("router", fmt.Sprintf("@@@@ Handling update %s", item), nil)
			}
			if len(update.Uaid) == 0 {
				continue
			}
			updater(&update)
		}
	}
	if err != nil {
		if self.Logger != nil {
			self.Logger.Error("updater", "Error: update: "+err.Error(), nil)
		}
	}
	conn.Close()
	return err
}

func (self *Router) SendUpdate(host, uaid, chid string, version int64) (err error) {

	var route *Route
	var ok bool

	if route, ok = routes[host]; !ok {
		// create a new route
		conn, err := net.Dial("tcp", host+":"+self.Port)
		if err != nil {
			return err
		}
		if self.Logger != nil {
			self.Logger.Info("router", "@@@ Creating new route to "+host, nil)
		}
		route = &Route{
			socket: conn,
		}
		routes[host] = route
	}

	data, err := json.Marshal(Update{
		Uaid: uaid,
		Chid: chid,
		Vers: version})
	if err != nil {
		return err
	}
	if self.Logger != nil {
		self.Logger.Debug("router", "@@@ Writing to host "+host, nil)
	}
	_, err = route.socket.Write([]byte(string(data) + "\n"))
	if err != nil {
		if self.Logger != nil {
			self.Logger.Error("router", "@@@ Closing socket to "+host, nil)
			log.Printf("ERROR: %s", err.Error())
		}
		route.socket.Close()
		delete(routes, host)
	}
	return err
}

func init() {
	routes = make(map[string]*Route)
}
