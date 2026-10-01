package main

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// tail keeps the last n bytes of s. ffmpeg prints the reason it stopped
// at the end of its output, after the description of the input.
func tail(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return "..." + s[len(s)-n:]
}
