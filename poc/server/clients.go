package server

import (
	"errors"

	"qas-poc/quark"
)

func (s *Server) clientFor(cookie string) *quark.Client {
	if s.newClient != nil {
		return s.newClient(cookie)
	}
	return quark.New(cookie)
}

func (s *Server) client() (*quark.Client, error) {
	snapshot := s.store.Snapshot()
	if len(snapshot.Cookies) == 0 {
		return nil, errors.New("未配置 Cookie，请先在设置中填写")
	}
	return s.clientFor(snapshot.Cookies[0]), nil
}

func apiMessage(err error) string {
	var apiErr *quark.APIError
	if errors.As(err, &apiErr) && apiErr.Message != "" {
		return apiErr.Message
	}
	return err.Error()
}
