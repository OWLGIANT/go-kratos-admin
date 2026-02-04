package lineswitcher

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"actor/broker/base"
	"actor/third/log"
	"github.com/labstack/echo/v4"
)

const (
	pathLineQuery  = "line_query"
	pathLineSwitch = "line_switch"
)

func init() {
	http.Handle("/exec/line_switch", &LineSwitcher)
}

func InitForEchoFramework(e *echo.Echo) {
	e.POST(fmt.Sprintf("/exec/%s", pathLineSwitch), LineSwitcher.handleReq)
	e.POST(fmt.Sprintf("/exec/%s", pathLineQuery), LineSwitcher.handleReq)
}

type LineSwitcher_T struct {
	RsSet sync.Map
}

func (s *LineSwitcher_T) handleReq(c echo.Context) error {
	var reqBody RequestBody
	err := json.NewDecoder(c.Request().Body).Decode(&reqBody)
	if err != nil {
		bodyStr, _ := io.ReadAll(c.Request().Body)
		return c.HTML(http.StatusBadRequest, fmt.Sprintf("failed to decode JSON body in line-switcher. req body: %s, err: %v", string(bodyStr), err))
	}

	if strings.HasSuffix(c.Request().RequestURI, pathLineSwitch) {
		rsp := s.switchLine(reqBody)
		if rsp.Code == 0 {
			return c.JSON(http.StatusOK, rsp)
		}
	} else if strings.HasSuffix(c.Request().RequestURI, pathLineQuery) {
		rsp := s.getLine(reqBody)
		if rsp.Code == 0 {
			return c.JSON(http.StatusOK, rsp)
		}
	}
	return c.HTML(http.StatusBadGateway, "failed to handle platform req in line-switcher")
}
func (s *LineSwitcher_T) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	var reqBody RequestBody
	err := json.NewDecoder(req.Body).Decode(&reqBody)
	if err != nil {
		http.Error(w, "Failed to decode JSON body", http.StatusBadRequest)
		return
	}

	rsp := s.switchLine(reqBody)

	w.Header().Add("Content-Type", "application/json")
	js, _ := json.Marshal(rsp)
	w.Write(js)
}
func (s *LineSwitcher_T) AddRs(rs base.Rs) {
	if rs == nil {
		log.Errorf("rs is nil, skip it")
		return
	}
	s.RsSet.Store(rs, rs.GetExName())
	rs.AddRsStopHandler(func(rs base.Rs) {
		log.Infof("remove rs in LineSwitcher. %p", &rs)
		s.RsSet.Delete(rs)
	})
}

type RequestBody struct {
	Ex     string
	Action string
	Client string
	Link   string
}

type ResultItem struct {
	Err     string
	Changed bool
	Ex      string
}
type QueryLineResultItem struct {
	Line string
	Ex   string
}

type RspBody[T any] struct {
	Code    int
	Msg     string
	Results []T
}

func (s *LineSwitcher_T) getLine(reqBody RequestBody) (rsp RspBody[QueryLineResultItem]) {
	if reqBody.Ex == "" {
		rsp.Code = 1
		rsp.Msg = "must provide ex"
		return
	}

	var action base.ActionType
	if reqBody.Action == base.ActionType_Cancel.String() {
		action = base.ActionType_Cancel
	} else if reqBody.Action == base.ActionType_Place.String() {
		action = base.ActionType_Place
	} else if reqBody.Action == base.ActionType_Amend.String() {
		action = base.ActionType_Amend
	} else {
		rsp.Code = 1
		rsp.Msg = "must provide action"
		return
	}

	rsp.Results = make([]QueryLineResultItem, 0)

	s.RsSet.Range(func(key, value any) bool {
		ex, _ := value.(string)
		if ex == reqBody.Ex {
			rs, _ := key.(base.Rs)
			l := rs.GetSelectedLine(action)
			rsp.Results = append(rsp.Results, QueryLineResultItem{
				Line: l.String(),
				Ex:   ex,
			})
		}
		return true
	})
	return
}

func (s *LineSwitcher_T) switchLine(reqBody RequestBody) (rsp RspBody[ResultItem]) {
	if reqBody.Ex == "" {
		rsp.Code = 1
		rsp.Msg = "must provide ex"
		return
	}

	var action base.ActionType
	if reqBody.Action == base.ActionType_Cancel.String() {
		action = base.ActionType_Cancel
	} else if reqBody.Action == base.ActionType_Place.String() {
		action = base.ActionType_Place
	} else if reqBody.Action == base.ActionType_Amend.String() {
		action = base.ActionType_Amend
	} else {
		rsp.Code = 1
		rsp.Msg = "must provide action"
		return
	}

	rsp.Results = make([]ResultItem, 0)

	s.RsSet.Range(func(key, value any) bool {
		ex, _ := value.(string)
		if ex == reqBody.Ex {
			rs, _ := key.(base.Rs)
			err, ok := rs.SwitchLine(action, base.SelectedLine{Line: base.Line{Client: base.ClientType(reqBody.Client), Link: base.LinkType(reqBody.Link)}})
			errStr := ""
			if err != nil {
				errStr = err.Error()
			}
			log.Infof("switchline. ok: %v, err: %v", ok, err)
			rsp.Results = append(rsp.Results, ResultItem{
				Err:     errStr,
				Changed: ok,
				Ex:      ex,
			})
		}
		return true
	})
	return
}

var LineSwitcher LineSwitcher_T
