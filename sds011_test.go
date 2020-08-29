package sds011

import (
	"fmt"
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
			[]byte{0xaa, 0xc0, 0x2d, 0x00, 0xb8, 0x00, 0x54, 0x6f, 0xa8, 0xab},
			Measurement{
				PM25: 4.5,
				PM10: 18.4,
			},
		},
		{
			"zero",
			[]byte{0xaa, 0xc0, 0x00, 0x00, 0x00, 0x00, 0x54, 0x6f, 0xc3, 0xab},
			Measurement{
				PM25: 0,
				PM10: 0,
			},
		},
		{
			"pm25 only",
			[]byte{0xaa, 0xc0, 0x2d, 0x00, 0x00, 0x00, 0x54, 0x6f, 0xf0, 0xab},
			Measurement{
				PM25: 4.5,
				PM10: 0,
			},
		},
		{
			"pm10 only",
			[]byte{0xaa, 0xc0, 0x00, 0x00, 0xb8, 0x00, 0x54, 0x6f, 0x7b, 0xab},
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
				t.Errorf("Unexpected result (-want +got):\n%s", diff)
			}
		})
	}
}

func TestValidateFailures(t *testing.T) {
	cases := []struct {
		name    string
		buf     []byte
		errText string
	}{
		{
			"length",
			[]byte{0xaa, 0xc0, 0x2d, 0x00, 0xb8, 0x00, 0x54, 0x6f, 0xa8},
			"bad packet length",
		},
		{
			"header",
			[]byte{0xab, 0xc0, 0x2d, 0x00, 0xb8, 0x00, 0x54, 0x6f, 0xa8, 0xab},
			"bad header",
		},
		{
			"tail",
			[]byte{0xaa, 0xc0, 0x2d, 0x00, 0xb8, 0x00, 0x54, 0x6f, 0xa8, 0xac},
			"bad tail",
		},
		{
			"checksum",
			[]byte{0xaa, 0xc0, 0x2d, 0x00, 0xb8, 0x00, 0x54, 0x6f, 0xa9, 0xab},
			"bad checksum",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validate(tc.buf, cmdTypeQuery, 0x2d)
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

func TestChecksum(t *testing.T) {
	cases := []struct {
		b    []byte
		want byte
	}{
		{
			b:    nil,
			want: 0x00,
		},
		{
			b:    []byte{},
			want: 0x00,
		},
		{
			b:    []byte{0x36},
			want: 0x36,
		},
		{
			b:    []byte{0xaa, 0xbb, 0xcc, 0xdd, 0xff, 0x08, 0x07, 0xff, 0xff},
			want: 0x1a,
		},
	}

	for _, tc := range cases {
		t.Run(fmt.Sprintf("%v", tc.b), func(t *testing.T) {
			got := checksum(tc.b)
			if got != tc.want {
				t.Errorf("got 0x%x, want 0x%x", got, tc.want)
			}
		})
	}
}

func TestToBytes(t *testing.T) {
	got := toBytes(0xabcd)
	want := []byte{0xab, 0xcd}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Unexpected result (-want +got):\n%s", diff)
	}
}

func TestContains(t *testing.T) {
	cases := []struct {
		s    []byte
		b    byte
		want bool
	}{
		{
			s:    nil,
			b:    0xaa,
			want: false,
		},
		{
			s:    []byte{},
			b:    0xaa,
			want: false,
		},
		{
			s:    []byte{0x36},
			b:    0xaa,
			want: false,
		},
		{
			s:    []byte{0xaa, 0xbb, 0xcc, 0xdd, 0xff, 0x08, 0x07, 0xff, 0xff},
			b:    0x08,
			want: true,
		},
		{
			s:    []byte{0xaa, 0xbb, 0xcc, 0xdd, 0xff, 0x08, 0x07, 0xff, 0xff},
			b:    0x99,
			want: false,
		},
	}

	for _, tc := range cases {
		t.Run("", func(t *testing.T) {
			got := contains(tc.s, tc.b)
			if got != tc.want {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestFmtBytes(t *testing.T) {
	cases := []struct {
		b    []byte
		want string
	}{
		{
			b:    nil,
			want: "<nil>",
		},
		{
			b:    []byte{},
			want: "[]",
		},
		{
			b:    []byte{0xab},
			want: "[0xab]",
		},
		{
			b:    []byte{0xab, 0xff},
			want: "[0xab, 0xff]",
		},
		{
			b:    []byte{0xab, 0xac, 0xff},
			want: "[0xab, 0xac, 0xff]",
		},
	}

	for _, tc := range cases {
		t.Run(fmt.Sprintf("%v", tc.b), func(t *testing.T) {
			got := fmtBytes(tc.b)
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}
