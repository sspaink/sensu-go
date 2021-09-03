package pipeline

import (
	"context"
	"reflect"
	"testing"
	"time"

	corev2 "github.com/sensu/sensu-go/api/core/v2"
	"github.com/sensu/sensu-go/backend/pipeline/handler"
	"github.com/sensu/sensu-go/backend/pipeline/mutator"
	"github.com/sensu/sensu-go/backend/store"
	"github.com/sensu/sensu-go/command"
	"github.com/sensu/sensu-go/testing/mockexecutor"
	"github.com/sensu/sensu-go/testing/mockstore"
	"github.com/stretchr/testify/mock"
)

func TestAdapterV1_Name(t *testing.T) {
	o := &AdapterV1{}
	want := "AdapterV1"

	if got := o.Name(); want != got {
		t.Errorf("AdapterV1.Name() = %v, want %v", got, want)
	}
}

func TestAdapterV1_CanRun(t *testing.T) {
	type fields struct {
		Store           store.Store
		StoreTimeout    time.Duration
		FilterAdapters  []FilterAdapter
		MutatorAdapters []MutatorAdapter
		HandlerAdapters []HandlerAdapter
	}
	type args struct {
		ref *corev2.ResourceReference
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
	}{
		{
			name: "returns false when resource reference is not a core/v2.Pipeline",
			args: args{
				ref: &corev2.ResourceReference{
					APIVersion: "core/v2",
					Type:       "Handler",
				},
			},
			want: false,
		},
		{
			name: "returns true when resource reference is a core/v2.Pipeline",
			args: args{
				ref: &corev2.ResourceReference{
					APIVersion: "core/v2",
					Type:       "Pipeline",
				},
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &AdapterV1{
				Store:           tt.fields.Store,
				StoreTimeout:    tt.fields.StoreTimeout,
				FilterAdapters:  tt.fields.FilterAdapters,
				MutatorAdapters: tt.fields.MutatorAdapters,
				HandlerAdapters: tt.fields.HandlerAdapters,
			}
			if got := a.CanRun(tt.args.ref); got != tt.want {
				t.Errorf("AdapterV1.CanRun() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAdapterV1_Run(t *testing.T) {
	type fields struct {
		Store           store.Store
		StoreTimeout    time.Duration
		FilterAdapters  []FilterAdapter
		MutatorAdapters []MutatorAdapter
		HandlerAdapters []HandlerAdapter
	}
	type args struct {
		ctx      context.Context
		ref      *corev2.ResourceReference
		resource interface{}
	}
	tests := []struct {
		name       string
		fields     fields
		args       args
		wantErr    bool
		wantErrMsg string
	}{
		{
			name: "returns error when resource is not a core/v2.Event",
			args: args{
				resource: corev2.FixtureHandler("handler1"),
			},
			wantErr:    true,
			wantErrMsg: "resource is not a corev2.Event",
		},
		{
			name: "returns error when the store returns an error",
			args: args{
				ctx:      context.Background(),
				ref:      corev2.FixturePipelineReference("pipeline1"),
				resource: corev2.FixtureEvent("entity1", "check1"),
			},
			fields: fields{
				Store: func() store.Store {
					var pipeline *corev2.Pipeline
					err := &store.ErrInternal{Message: "etcd timeout"}
					stor := &mockstore.MockStore{}
					stor.On("GetPipelineByName", mock.Anything, "pipeline1").Return(pipeline, err)
					return stor
				}(),
			},
			wantErr:    true,
			wantErrMsg: "internal error: etcd timeout",
		},
		{
			name: "returns error when pipeline does not exist",
			args: args{
				ctx:      context.Background(),
				ref:      corev2.FixturePipelineReference("pipeline1"),
				resource: corev2.FixtureEvent("entity1", "check1"),
			},
			fields: fields{
				Store: func() store.Store {
					var pipeline *corev2.Pipeline
					stor := &mockstore.MockStore{}
					stor.On("GetPipelineByName", mock.Anything, "pipeline1").Return(pipeline, nil)
					return stor
				}(),
			},
			wantErr:    true,
			wantErrMsg: "pipeline does not exist",
		},
		{
			name: "returns error when pipeline has no workflows",
			args: args{
				ctx:      context.Background(),
				ref:      corev2.FixturePipelineReference("pipeline1"),
				resource: corev2.FixtureEvent("entity1", "check1"),
			},
			fields: fields{
				Store: func() store.Store {
					pipeline := &corev2.Pipeline{
						ObjectMeta: corev2.NewObjectMeta("pipeline1", "default"),
						Workflows:  []*corev2.PipelineWorkflow{},
					}
					stor := &mockstore.MockStore{}
					stor.On("GetPipelineByName", mock.Anything, pipeline.GetName()).Return(pipeline, nil)
					return stor
				}(),
			},
			wantErr:    true,
			wantErrMsg: "pipeline has no workflows",
		},
		{
			name: "returns error when handler produces an error",
			args: args{
				ctx:      context.Background(),
				ref:      corev2.FixturePipelineReference("pipeline1"),
				resource: corev2.FixtureEvent("entity1", "check1"),
			},
			fields: fields{
				MutatorAdapters: []MutatorAdapter{
					&mutator.JSONAdapter{},
				},
				HandlerAdapters: func() []HandlerAdapter {
					var nilHandler *corev2.Handler
					err := &store.ErrInternal{Message: "etcd timeout"}
					stor := &mockstore.MockStore{}
					stor.On("GetHandlerByName", mock.Anything, mock.Anything).
						Return(nilHandler, err)
					ex := &mockexecutor.MockExecutor{}
					execution := command.FixtureExecutionResponse(0, "foo")
					ex.Return(execution, nil)
					return []HandlerAdapter{
						&handler.LegacyAdapter{
							Store:    stor,
							Executor: ex,
						},
					}
				}(),
				Store: func() store.Store {
					pipeline := &corev2.Pipeline{
						ObjectMeta: corev2.NewObjectMeta("pipeline1", "default"),
						Workflows: []*corev2.PipelineWorkflow{
							{
								Name:    "send metrics to prometheus",
								Filters: nil,
								Mutator: nil,
								Handler: &corev2.ResourceReference{
									APIVersion: "core/v2",
									Type:       "Handler",
									Name:       "handler1",
								},
							},
						},
					}
					stor := &mockstore.MockStore{}
					stor.On("GetPipelineByName", mock.Anything, pipeline.GetName()).Return(pipeline, nil)
					return stor
				}(),
			},
			wantErr:    true,
			wantErrMsg: "failed to fetch handler from store: internal error: etcd timeout",
		},
		{
			name: "returns nil when pipeline successfully runs",
			args: args{
				ctx:      context.Background(),
				ref:      corev2.FixturePipelineReference("pipeline1"),
				resource: corev2.FixtureEvent("entity1", "check1"),
			},
			fields: fields{
				MutatorAdapters: []MutatorAdapter{
					&mutator.JSONAdapter{},
				},
				HandlerAdapters: func() []HandlerAdapter {
					storedHandler := corev2.FixtureHandler("handler1")
					stor := &mockstore.MockStore{}
					stor.On("GetHandlerByName", mock.Anything, storedHandler.GetName()).Return(storedHandler, nil)
					ex := &mockexecutor.MockExecutor{}
					execution := command.FixtureExecutionResponse(0, "foo")
					ex.Return(execution, nil)
					return []HandlerAdapter{
						&handler.LegacyAdapter{
							Store:    stor,
							Executor: ex,
						},
					}
				}(),
				Store: func() store.Store {
					pipeline := &corev2.Pipeline{
						ObjectMeta: corev2.NewObjectMeta("pipeline1", "default"),
						Workflows: []*corev2.PipelineWorkflow{
							{
								Name:    "send metrics to prometheus",
								Filters: nil,
								Mutator: nil,
								Handler: &corev2.ResourceReference{
									APIVersion: "core/v2",
									Type:       "Handler",
									Name:       "handler1",
								},
							},
						},
					}
					stor := &mockstore.MockStore{}
					stor.On("GetPipelineByName", mock.Anything, pipeline.GetName()).Return(pipeline, nil)
					return stor
				}(),
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &AdapterV1{
				Store:           tt.fields.Store,
				StoreTimeout:    tt.fields.StoreTimeout,
				FilterAdapters:  tt.fields.FilterAdapters,
				MutatorAdapters: tt.fields.MutatorAdapters,
				HandlerAdapters: tt.fields.HandlerAdapters,
			}
			err := a.Run(tt.args.ctx, tt.args.ref, tt.args.resource)
			if (err != nil) != tt.wantErr {
				t.Errorf("AdapterV1.Run() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && err.Error() != tt.wantErrMsg {
				t.Errorf("AdapterV1.Run() error msg = %v, wantErrMsg %v", err.Error(), tt.wantErrMsg)
			}
		})
	}
}

func TestAdapterV1_resolvePipelineReference(t *testing.T) {
	type fields struct {
		Store           store.Store
		StoreTimeout    time.Duration
		FilterAdapters  []FilterAdapter
		MutatorAdapters []MutatorAdapter
		HandlerAdapters []HandlerAdapter
	}
	type args struct {
		ctx   context.Context
		ref   *corev2.ResourceReference
		event *corev2.Event
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *corev2.Pipeline
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &AdapterV1{
				Store:           tt.fields.Store,
				StoreTimeout:    tt.fields.StoreTimeout,
				FilterAdapters:  tt.fields.FilterAdapters,
				MutatorAdapters: tt.fields.MutatorAdapters,
				HandlerAdapters: tt.fields.HandlerAdapters,
			}
			got, err := a.resolvePipelineReference(tt.args.ctx, tt.args.ref, tt.args.event)
			if (err != nil) != tt.wantErr {
				t.Errorf("AdapterV1.resolvePipelineReference() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("AdapterV1.resolvePipelineReference() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAdapterV1_getPipelineFromStore(t *testing.T) {
	type fields struct {
		Store           store.Store
		StoreTimeout    time.Duration
		FilterAdapters  []FilterAdapter
		MutatorAdapters []MutatorAdapter
		HandlerAdapters []HandlerAdapter
	}
	type args struct {
		ctx context.Context
		ref *corev2.ResourceReference
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *corev2.Pipeline
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &AdapterV1{
				Store:           tt.fields.Store,
				StoreTimeout:    tt.fields.StoreTimeout,
				FilterAdapters:  tt.fields.FilterAdapters,
				MutatorAdapters: tt.fields.MutatorAdapters,
				HandlerAdapters: tt.fields.HandlerAdapters,
			}
			got, err := a.getPipelineFromStore(tt.args.ctx, tt.args.ref)
			if (err != nil) != tt.wantErr {
				t.Errorf("AdapterV1.getPipelineFromStore() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("AdapterV1.getPipelineFromStore() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAdapterV1_generateLegacyPipeline(t *testing.T) {
	type fields struct {
		Store           store.Store
		StoreTimeout    time.Duration
		FilterAdapters  []FilterAdapter
		MutatorAdapters []MutatorAdapter
		HandlerAdapters []HandlerAdapter
	}
	type args struct {
		ctx   context.Context
		event *corev2.Event
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *corev2.Pipeline
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &AdapterV1{
				Store:           tt.fields.Store,
				StoreTimeout:    tt.fields.StoreTimeout,
				FilterAdapters:  tt.fields.FilterAdapters,
				MutatorAdapters: tt.fields.MutatorAdapters,
				HandlerAdapters: tt.fields.HandlerAdapters,
			}
			got, err := a.generateLegacyPipeline(tt.args.ctx, tt.args.event)
			if (err != nil) != tt.wantErr {
				t.Errorf("AdapterV1.generateLegacyPipeline() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("AdapterV1.generateLegacyPipeline() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAdapterV1_expandHandlers(t *testing.T) {
	type fields struct {
		Store           store.Store
		StoreTimeout    time.Duration
		FilterAdapters  []FilterAdapter
		MutatorAdapters []MutatorAdapter
		HandlerAdapters []HandlerAdapter
	}
	type args struct {
		ctx      context.Context
		handlers []string
		level    int
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    HandlerMap
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &AdapterV1{
				Store:           tt.fields.Store,
				StoreTimeout:    tt.fields.StoreTimeout,
				FilterAdapters:  tt.fields.FilterAdapters,
				MutatorAdapters: tt.fields.MutatorAdapters,
				HandlerAdapters: tt.fields.HandlerAdapters,
			}
			got, err := a.expandHandlers(tt.args.ctx, tt.args.handlers, tt.args.level)
			if (err != nil) != tt.wantErr {
				t.Errorf("AdapterV1.expandHandlers() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("AdapterV1.expandHandlers() = %v, want %v", got, tt.want)
			}
		})
	}
}
