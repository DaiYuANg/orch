package model

type Response[T any] struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    T      `json:"data,omitempty"`
}

func WrapResponse[T any](data T) *struct {
	Body Response[T]
} {
	return &struct {
		Body Response[T]
	}{
		Body: Response[T]{
			Code:    0,
			Message: "ok",
			Data:    data,
		},
	}
}
