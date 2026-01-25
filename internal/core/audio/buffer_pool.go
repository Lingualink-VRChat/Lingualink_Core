package audio

import "sync"

const (
	defaultAudioBufferCap = 1024 * 1024      // 1MB
	maxPooledAudioCap     = 8 * 1024 * 1024  // 8MB
	maxAudioSizeBytes     = 32 * 1024 * 1024 // 32MB (request limit)
)

var audioBufferPool = sync.Pool{
	New: func() any {
		buf := make([]byte, 0, defaultAudioBufferCap)
		return buf
	},
}

// AcquireAudioBuffer returns a byte slice with length minLen and at least minLen capacity.
// The returned buffer is owned by the caller and should be returned with ReleaseAudioBuffer.
func AcquireAudioBuffer(minLen int) []byte {
	if minLen < 0 {
		minLen = 0
	}
	if minLen > maxAudioSizeBytes {
		return make([]byte, minLen)
	}

	buf := audioBufferPool.Get().([]byte)
	if cap(buf) < minLen {
		buf = make([]byte, minLen)
	} else {
		buf = buf[:minLen]
	}
	return buf
}

// ReleaseAudioBuffer returns a buffer acquired from AcquireAudioBuffer back to the pool.
func ReleaseAudioBuffer(buf []byte) {
	if buf == nil {
		return
	}
	if cap(buf) > maxPooledAudioCap {
		return
	}
	audioBufferPool.Put(buf[:0])
}
