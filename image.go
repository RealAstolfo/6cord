package main

import (
	"net/http"
	"time"

	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	"github.com/diamondburned/discordgo"
	img "gitlab.com/diamondburned/6cord/image"
)

type imageCtx struct {
	state img.Backend
	index int
}

type imageRendererPipelineStruct struct {
	event   chan interface{}
	state   img.Backend
	message int64
	index   int

	cache  *imageCacheStruct
	assets []*imageCacheAsset
}

const (
	imagePipelineNextEvent int = iota
	imagePipelinePrevEvent
)

var imageRendererPipeline = startImageRendererPipeline()

func startImageRendererPipeline() *imageRendererPipelineStruct {
	p := &imageRendererPipelineStruct{
		event: make(chan interface{}, 5),
		cache: &imageCacheStruct{
			client: &http.Client{
				Timeout: 30 * time.Second,
			},
		},
	}

	go func() {
		for i := range p.event {
		Switch:
			switch i := i.(type) {
			case *discordgo.Message:
				p.message = i.ID

				p.assets = p.cache.get(i.ID)
				if p.assets == nil {
					var err error

					p.assets, err = p.cache.set(i)
					if err != nil {
						Warn(err.Error())
						break
					}
				}

				p.clean()

				if p.assets == nil {
					break Switch
				}

				p.show()

			case int:
				if p.assets == nil {
					break Switch
				}

				switch i {
				case imagePipelineNextEvent:
					p.index++
					if p.index >= len(p.assets) {
						p.index = 0
					}
				case imagePipelinePrevEvent:
					p.index--
					if p.index < 0 {
						p.index = len(p.assets) - 1
					}
				default:
					break Switch
				}

				p.show()

			default:
				break Switch
			}
		}
	}()

	return p
}

func (p *imageRendererPipelineStruct) add(m *discordgo.Message) {
	p.event <- m
}

func (p *imageRendererPipelineStruct) next() {
	p.event <- imagePipelineNextEvent
}

func (p *imageRendererPipelineStruct) prev() {
	p.event <- imagePipelinePrevEvent
}

func (p *imageRendererPipelineStruct) clean() {
	if p.state != nil {
		p.state.Delete()
	}
}

func (p *imageRendererPipelineStruct) show() (err error) {
	p.clean()

	if p.assets == nil {
		return nil
	}

	p.state, err = img.New(p.assets[p.index].i)
	if err != nil {
		return err
	}

	return nil
}