package text

import "sync"

var processResponsePool = sync.Pool{
	New: func() any {
		return &ProcessResponse{
			Translations: make(map[string]string),
			Metadata:     make(map[string]interface{}),
		}
	},
}

func acquireProcessResponse() *ProcessResponse {
	resp := processResponsePool.Get().(*ProcessResponse)
	resp.RequestID = ""
	resp.Status = ""
	resp.SourceText = ""
	resp.CorrectedText = ""
	resp.RawResponse = ""
	resp.ProcessingTime = 0
	for k := range resp.Translations {
		delete(resp.Translations, k)
	}
	for k := range resp.Metadata {
		delete(resp.Metadata, k)
	}
	return resp
}

// Release resets the response and returns it to the pool.
// It should be called after the response has been serialized and written.
func (r *ProcessResponse) Release() {
	if r == nil {
		return
	}
	r.RequestID = ""
	r.Status = ""
	r.SourceText = ""
	r.CorrectedText = ""
	r.RawResponse = ""
	r.ProcessingTime = 0
	for k := range r.Translations {
		delete(r.Translations, k)
	}
	for k := range r.Metadata {
		delete(r.Metadata, k)
	}
	processResponsePool.Put(r)
}
