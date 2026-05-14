package gossipsvc

type delegate struct {
	service *Service
}

func (d delegate) NodeMeta(limit int) []byte {
	if d.service == nil {
		return nil
	}
	return d.service.nodeMeta(limit)
}

func (delegate) NotifyMsg([]byte) {}

func (delegate) GetBroadcasts(_, _ int) [][]byte {
	return nil
}

func (delegate) LocalState(bool) []byte {
	return nil
}

func (delegate) MergeRemoteState([]byte, bool) {}
