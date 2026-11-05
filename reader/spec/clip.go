package spec

import "github.com/readium/go-toolkit/pkg/guidednavigation"

// What a guided navigation object (a SMIL par or seq) says about its audio.
type Audio int

const (
	NoAudio   Audio = iota // no audio reference: a group, or text for text-to-speech
	Clipped                // a clip with its end: File, Begin and End are set
	OpenEnded              // audio with no clipEnd, or no clip at all: it runs to the end of the file (not supported yet)
)

// Clip reads an object's audio as the spec has it (Reading Systems 3.3, 9.2.2): no clipBegin is 0, no clipEnd is the end
// of the file. It never fails on an object without audio.
func Clip(o guidednavigation.GuidedNavigationObject) (file string, begin, end float64, kind Audio) {
	if o.AudioRef == nil {
		return "", 0, 0, NoAudio
	}
	f := o.AudioFile()
	if f == nil || f.String() == "" {
		return "", 0, 0, NoAudio
	}
	file = f.String()
	c := o.AudioClip()
	if c == nil || c.End == nil {
		return file, 0, 0, OpenEnded
	}
	if c.Begin != nil {
		begin = c.Begin.Seconds()
	}
	return file, begin, c.End.Seconds(), Clipped
}
