package machine

type inChannel struct {
	channel chan Payload
}

type outChannel struct {
	channel chan Payload
}

func newInChannel() *inChannel {
	return &inChannel{
		make(chan Payload),
	}
}

func newOutChannel() *outChannel {
	return &outChannel{
		make(chan Payload),
	}
}

func (out *outChannel) convert() *inChannel {
	return &inChannel{
		channel: out.channel,
	}
}

func (out *outChannel) sendTo(in *inChannel) {
	go func() {
		for {
			list := <-out.channel
			if len(list) > 0 {
				in.channel <- list
			}
		}
	}()
}
