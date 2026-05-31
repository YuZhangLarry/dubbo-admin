package agent

import (
	"context"
	"fmt"

	"dubbo-admin-ai/component/memory"
	"dubbo-admin-ai/schema"

	"github.com/firebase/genkit/go/core"
)

type NoStream = struct{}
type StreamType interface {
	NoStream | schema.StreamChunk
}

type Flow = *core.Flow[schema.Schema, schema.Schema, any]
type NormalFlow = *core.Flow[schema.Schema, schema.Schema, NoStream]
type StreamFlow = *core.Flow[schema.Schema, schema.Schema, schema.StreamChunk]

type StreamHandler = func(*core.StreamingFlowValue[schema.Schema, schema.StreamChunk], error) bool
type StreamFunc = func(*Channels) StreamHandler

const (
	IntentFlowName  string = "intent"
	ThinkFlowName   string = "think"
	ActFlowName     string = "act"
	ObserveFlowName string = "observe"
	ReActFlowName   string = "reAct"
)

type Agent interface {
	Interact(*schema.UserInput, string) *Channels
	GetMemory() *memory.HistoryMemory
}

type Channels struct {
	closed bool

	UserRespChan chan *schema.StreamFeedback
	FlowChan     chan schema.Schema
	ErrorChan    chan error
}

func NewChannels(bufferSize int) *Channels {
	return &Channels{
		closed:       false,
		UserRespChan: make(chan *schema.StreamFeedback, bufferSize),
		FlowChan:     make(chan schema.Schema, bufferSize),
		ErrorChan:    make(chan error, bufferSize),
	}
}

func (chans *Channels) Reset() {
	chans.closed = false
}

// This method won't destroy the Channels because it will be reused for each interaction.
// If you want to completely destroy the Channels, please call Destroy() method.
func (chans *Channels) Close() {
	chans.closed = true
}

func (chans *Channels) Closed() bool {
	return chans.closed
}

func (chans *Channels) Destroy() {
	close(chans.UserRespChan)
	close(chans.FlowChan)
	close(chans.ErrorChan)
	chans = nil
}

type StageType int

const (
	BeforeLoop StageType = iota
	InLoop
	AfterLoop
)

type Stage struct {
	flow       any
	streamFunc StreamFunc
	Type       StageType
}

func NewStage(flow any, t StageType) *Stage {
	return &Stage{
		flow:       flow,
		streamFunc: nil,
		Type:       t,
	}
}

func NewStreamStage(flow any, t StageType, onStreaming func(*Channels, schema.StreamChunk) error, onDone func(*Channels, schema.Schema) error) (stage *Stage) {
	if onStreaming == nil || onDone == nil {
		panic("onStreaming, onDone and streamChan callbacks cannot be nil for streaming stage")
	}

	stage = NewStage(flow, t)
	stage.streamFunc = func(channels *Channels) StreamHandler {
		return func(val *core.StreamingFlowValue[schema.Schema, schema.StreamChunk], err error) bool {
			if err != nil {
				channels.ErrorChan <- err
				return false
			}
			if !val.Done {
				if err := onStreaming(channels, val.Stream); err != nil {
					channels.ErrorChan <- err
					return false
				}
			} else if val.Output != nil {
				if err := onDone(channels, val.Output); err != nil {
					channels.ErrorChan <- err
					return false
				}
			}
			return true
		}
	}
	return stage
}

// Execute will receive an input and produce an output
func (s *Stage) Execute(ctx context.Context, chans *Channels, input schema.Schema) error {
	if value, ok := s.flow.(NormalFlow); ok {
		output, err := value.Run(ctx, input)
		if err != nil {
			return fmt.Errorf("error when running normal flow: %w", err)
		}
		chans.FlowChan <- output
	} else if value, ok := s.flow.(StreamFlow); ok {
		if chans == nil || s.streamFunc == nil {
			return fmt.Errorf("stream handler and channels cannot be nil for stream flow")
		}
		// the stream function will produce the output
		value.Stream(ctx, input)(s.streamFunc(chans))
	}

	return nil
}

func emitStageProgress(chans *Channels, stageName string, started bool) {
	if chans == nil {
		return
	}
	chans.UserRespChan <- schema.NewStreamFeedback(stageProgressText(stageName, started))
}

func stageProgressText(stageName string, started bool) string {
	if started {
		switch stageName {
		case ThinkFlowName:
			return "🔍 分析问题中...\n"
		case ActFlowName:
			return "🛠️ 正在调用工具...\n"
		case ObserveFlowName:
			return "🧠 整理结论中...\n"
		default:
			return fmt.Sprintf("⏳ %s 阶段处理中...\n", stageName)
		}
	}

	switch stageName {
	case ThinkFlowName:
		return "✅ 问题分析完成。\n"
	case ActFlowName:
		return "✅ 工具调用完成。\n"
	case ObserveFlowName:
		return "✅ 结论整理完成。\n"
	default:
		return fmt.Sprintf("✅ %s 阶段完成。\n", stageName)
	}
}
