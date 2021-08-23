package filter

import (
	"context"
	"testing"

	corev2 "github.com/sensu/sensu-go/api/core/v2"
)

func TestHasMetrics_Name(t *testing.T) {
	o := &HasMetrics{}
	want := "HasMetrics"

	if got := o.Name(); want != got {
		t.Errorf("HasMetrics.Name() = %v, want %v", got, want)
	}
}

func TestHasMetrics_CanFilter(t *testing.T) {
	type args struct {
		ctx context.Context
		ref *corev2.ResourceReference
	}
	tests := []struct {
		name string
		i    *HasMetrics
		args args
		want bool
	}{
		{
			name: "returns false when resource reference is not a core/v2.EventFilter",
			args: args{
				ref: &corev2.ResourceReference{
					APIVersion: "core/v2",
					Type:       "Handler",
				},
			},
			want: false,
		},
		{
			name: "returns false when resource reference is a core/v2.EventFilter and its name is not has_metrics",
			args: args{
				ref: &corev2.ResourceReference{
					APIVersion: "core/v2",
					Type:       "EventFilter",
					Name:       "is_incident",
				},
			},
			want: false,
		},
		{
			name: "returns true when resource reference is a core/v2.EventFilter and its name is has_metrics",
			args: args{
				ref: &corev2.ResourceReference{
					APIVersion: "core/v2",
					Type:       "EventFilter",
					Name:       "has_metrics",
				},
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			i := &HasMetrics{}
			if got := i.CanFilter(tt.args.ctx, tt.args.ref); got != tt.want {
				t.Errorf("HasMetrics.CanFilter() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHasMetrics_Filter(t *testing.T) {
	type args struct {
		ctx   context.Context
		ref   *corev2.ResourceReference
		event *corev2.Event
	}
	tests := []struct {
		name    string
		i       *HasMetrics
		args    args
		want    bool
		wantErr bool
	}{
		{
			name: "event is denied when it does not have metrics",
			args: args{
				ctx: context.Background(),
				event: func() *corev2.Event {
					event := corev2.FixtureEvent("default", "default")
					return event
				}(),
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "event is allowed when it has metrics",
			args: args{
				ctx: context.Background(),
				event: func() *corev2.Event {
					event := corev2.FixtureEvent("default", "default")
					event.Metrics = corev2.FixtureMetrics()
					return event
				}(),
			},
			want:    false,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			i := &HasMetrics{}
			got, err := i.Filter(tt.args.ctx, tt.args.ref, tt.args.event)
			if (err != nil) != tt.wantErr {
				t.Errorf("HasMetrics.Filter() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("HasMetrics.Filter() = %v, want %v", got, tt.want)
			}
		})
	}
}
