package youtube

import "testing"

func TestIsPlaylistURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want bool
	}{
		{
			name: "playlist page URL",
			url:  "https://www.youtube.com/playlist?list=PLnBo22JgfHIZMrQmyTVSyx4q0bW_uwfyw",
			want: true,
		},
		{
			name: "watch URL with list param",
			url:  "https://www.youtube.com/watch?v=WZIGwN-5Ioo&list=PLnBo22JgfHIZMrQmyTVSyx4q0bW_uwfyw",
			want: true,
		},
		{
			name: "single video URL",
			url:  "https://www.youtube.com/watch?v=WZIGwN-5Ioo",
			want: false,
		},
		{
			name: "youtu.be short URL without list",
			url:  "https://youtu.be/WZIGwN-5Ioo",
			want: false,
		},
		{
			name: "youtu.be short URL with list",
			url:  "https://youtu.be/WZIGwN-5Ioo?list=PLnBo22JgfHIZMrQmyTVSyx4q0bW_uwfyw",
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsPlaylistURL(tt.url)
			if got != tt.want {
				t.Errorf("IsPlaylistURL(%q) = %v, want %v", tt.url, got, tt.want)
			}
		})
	}
}
