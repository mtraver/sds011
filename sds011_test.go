package sds011

import (
	"math"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

var cmpFloats = cmp.Comparer(func(x, y float32) bool {
	return math.Abs(float64(x-y)) < 0.00001
})

func TestUnmarshal(t *testing.T) {
	cases := []struct {
		name string
		buf  []byte
		want Measurement
	}{
		{
			"normal",
			[]byte{'\xaa', '\xc0', '\x2d', '\x00', '\xb8', '\x00', '\x54', '\x6f', '\xa8', '\xab'},
			Measurement{
				PM25: 4.5,
				PM10: 18.4,
			},
		},
		{
			"zero",
			[]byte{'\xaa', '\xc0', '\x00', '\x00', '\x00', '\x00', '\x54', '\x6f', '\xc3', '\xab'},
			Measurement{
				PM25: 0,
				PM10: 0,
			},
		},
		{
			"pm25 only",
			[]byte{'\xaa', '\xc0', '\x2d', '\x00', '\x00', '\x00', '\x54', '\x6f', '\xf0', '\xab'},
			Measurement{
				PM25: 4.5,
				PM10: 0,
			},
		},
		{
			"pm10 only",
			[]byte{'\xaa', '\xc0', '\x00', '\x00', '\xb8', '\x00', '\x54', '\x6f', '\x7b', '\xab'},
			Measurement{
				PM25: 0,
				PM10: 18.4,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := unmarshal(tc.buf)
			if err != nil {
				t.Error(err)
				return
			}

			if diff := cmp.Diff(tc.want, got, cmpFloats); diff != "" {
				t.Errorf("Unexpected result (-got +want):\n%s", diff)
			}
		})
	}
}

func TestUnmarshalFailures(t *testing.T) {
	cases := []struct {
		name    string
		buf     []byte
		errText string
	}{
		{
			"length",
			[]byte{'\xaa', '\xc0', '\x2d', '\x00', '\xb8', '\x00', '\x54', '\x6f', '\xa8'},
			"bad packet length",
		},
		{
			"header",
			[]byte{'\xab', '\xc0', '\x2d', '\x00', '\xb8', '\x00', '\x54', '\x6f', '\xa8', '\xab'},
			"bad header",
		},
		{
			"command number",
			[]byte{'\xaa', '\xc1', '\x2d', '\x00', '\xb8', '\x00', '\x54', '\x6f', '\xa8', '\xab'},
			"bad command number",
		},
		{
			"tail",
			[]byte{'\xaa', '\xc0', '\x2d', '\x00', '\xb8', '\x00', '\x54', '\x6f', '\xa8', '\xac'},
			"bad tail",
		},
		{
			"checksum",
			[]byte{'\xaa', '\xc0', '\x2d', '\x00', '\xb8', '\x00', '\x54', '\x6f', '\xa9', '\xab'},
			"bad checksum",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := unmarshal(tc.buf)
			if err == nil {
				t.Error("want error, got nil")
				return
			}

			if !strings.Contains(err.Error(), tc.errText) {
				t.Errorf("want error with substring %q, got %q", tc.errText, err)
			}
		})
	}
}
