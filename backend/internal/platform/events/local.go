package events

import "context"

// Fanout — publikon te disa kanale me radhë; gabimi i parë e ndal (ngjarja riprovohet nga releja).
type Fanout []Publisher

func (f Fanout) Publish(ctx context.Context, ev Event) error {
	for _, p := range f {
		if err := p.Publish(ctx, ev); err != nil {
			return err
		}
	}
	return nil
}

// HandlerPublisher — VETËM development: ngjarja i jepet një përpunuesi në proces, në vend të
// SNS → SQS → worker. Në AWS konsumatorët e SQS bëjnë të njëjtën punë me të njëjtat funksione.
type HandlerPublisher struct {
	Name string
	Fn   func(ctx context.Context, ev Event) error
}

func (h HandlerPublisher) Publish(ctx context.Context, ev Event) error { return h.Fn(ctx, ev) }
