package rawcdp

import (
	"context"
)

// Batch can combine Client.Call and Client.Listen.
type Batch struct {
	prepares []func(context.Context, *Client)
	cleanups []func()
	runs     []func(context.Context, *Client) error
}

// Call is same as Client.Call.
func (s *Batch) Call(method string, params interface{}, result interface{}) {
	s.runs = append(s.runs, func(ctx context.Context, client *Client) error {
		return client.Call(ctx, method, params, result)
	})
}

// Listen is same as Client.Listen.
func (s *Batch) Listen(method string, params interface{}) {
	var listener Listener
	var canceler Canceler

	s.prepares = append(s.prepares, func(ctx context.Context, client *Client) {
		listener, canceler = client.Listen(method)
	})
	s.runs = append(s.runs, func(ctx context.Context, client *Client) error {
		return listener(ctx, params)
	})
	s.cleanups = append(s.cleanups, func() {
		canceler()
	})
}

// Run runs combined Client.Call and Client.Listen.
func (s *Batch) Run(ctx context.Context, client *Client) error {
	for _, fn := range s.prepares {
		fn(ctx, client)
	}
	defer func() {
		for _, fn := range s.cleanups {
			fn()
		}
	}()

	for _, fn := range s.runs {
		if err := fn(ctx, client); err != nil {
			return err
		}
	}
	return nil
}
