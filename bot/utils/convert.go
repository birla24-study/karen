package utils

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"main/utils/structs"
	"main/utils/task"
)

// ProgressFunc reports conversion progress.
type ProgressFunc func(phase string, done, total int64)

// CONVERSION FEATURE: Determine if source codec is lossy (rough heuristic by extension/codec name).
func isLossySource(ext string, codec string) bool {
	ext = strings.ToLower(ext)
	if ext == ".m4a" && (codec == "AAC" || strings.Contains(codec, "AAC") || strings.Contains(codec, "ATMOS")) {
		return true
	}
	if ext == ".mp3" || ext == ".opus" || ext == ".ogg" {
		return true
	}
	return false
}

func normalizeLyrics(lrc string) string {
	lrc = strings.ReplaceAll(lrc, "\r\n", "\n")
	lrc = strings.ReplaceAll(lrc, "\r", "\n")
	return lrc
}

// CONVERSION FEATURE: Build ffmpeg arguments for desired target.
func buildFFmpegArgs(ffmpegPath, inPath, outPath, targetFmt, extraArgs string, coverPath string, lyrics string) ([]string, error) {
	args := []string{"-y", "-i", inPath}
	if coverPath != "" {
		args = append(args, "-i", coverPath)
	}
	switch targetFmt {
	case "flac":
		if coverPath != "" {
			args = append(args,
				"-map", "0:a",
				"-map", "1:v",
				"-c:a", "flac",
				"-c:v", "copy",
				"-disposition:v", "attached_pic",
			)
		} else {
			args = append(args, "-map", "0:a", "-c:a", "flac")
		}
	case "mp3":
		args = append(args, "-vn")
		// VBR quality 2 ~ high quality
		args = append(args, "-c:a", "libmp3lame", "-qscale:a", "2")
	case "opus":
		args = append(args, "-vn")
		// Medium/high quality
		args = append(args, "-c:a", "libopus", "-b:a", "192k", "-vbr", "on")
	case "wav":
		args = append(args, "-vn")
		args = append(args, "-c:a", "pcm_s16le")
	case "copy":
		args = append(args, "-vn")
		// Just container copy (probably pointless for same container)
		args = append(args, "-c", "copy")
	default:
		return nil, fmt.Errorf("unsupported convert-format: %s", targetFmt)
	}
	if targetFmt == "flac" && lyrics != "" {
		args = append(args, "-metadata", fmt.Sprintf("LYRICS=%s", normalizeLyrics(lyrics)))
	}
	if extraArgs != "" {
		// naive split; for complex quoting you could enhance
		args = append(args, strings.Fields(extraArgs)...)
	}
	args = append(args, outPath)
	return args, nil
}

// ConvertIfNeeded performs post-download conversion when enabled.
func ConvertIfNeeded(track *task.Track, lrc string, cfg *structs.ConfigSet, coverPath string, progress ProgressFunc) {
	if cfg == nil {
		return
	}
	if !cfg.ConvertAfterDownload {
		return
	}
	if cfg.ConvertFormat == "" {
		return
	}
	srcPath := track.SavePath
	if srcPath == "" {
		return
	}
	ext := strings.ToLower(filepath.Ext(srcPath))
	targetFmt := strings.ToLower(cfg.ConvertFormat)

	// Map extension for output
	if targetFmt == "copy" {
		fmt.Println("Convert (copy) requested; skipping because it produces no new format.")
		return
	}

	if cfg.ConvertSkipIfSourceMatch {
		if ext == "."+targetFmt {
			fmt.Printf("Conversion skipped (already %s)\n", targetFmt)
			return
		}
	}

	outBase := strings.TrimSuffix(srcPath, ext)
	outPath := outBase + "." + targetFmt

	// Handle lossy -> lossless cases: optionally skip or warn
	if (targetFmt == "flac" || targetFmt == "wav") && isLossySource(ext, track.Codec) {
		if cfg.ConvertSkipLossyToLossless {
			fmt.Println("Skipping conversion: source appears lossy and target is lossless; configured to skip.")
			return
		}
		if cfg.ConvertWarnLossyToLossless {
			fmt.Println("Warning: Converting lossy source to lossless container will not improve quality.")
		}
	}

	if _, err := exec.LookPath(cfg.FFmpegPath); err != nil {
		fmt.Printf("ffmpeg not found at '%s'; skipping conversion.\n", cfg.FFmpegPath)
		return
	}

	if progress != nil {
		progress("Converting", 0, 0)
	}
	args, err := buildFFmpegArgs(cfg.FFmpegPath, srcPath, outPath, targetFmt, cfg.ConvertExtraArgs, coverPath, lrc)
	if err != nil {
		fmt.Println("Conversion config error:", err)
		return
	}

	fmt.Printf("Converting -> %s ...\n", targetFmt)
	cmd := exec.Command(cfg.FFmpegPath, args...)
	cmd.Stdout = nil
	cmd.Stderr = nil
	start := time.Now()
	if err := cmd.Run(); err != nil {
		fmt.Println("Conversion failed:", err)
		// leave original
		return
	}
	fmt.Printf("Conversion completed in %s: %s\n", time.Since(start).Truncate(time.Millisecond), filepath.Base(outPath))

	if targetFmt == "flac" {
		if coverPath != "" {
			fmt.Printf("[cover] Embedded cover art in FLAC: %s\n", filepath.Base(outPath))
		}
		if lrc != "" {
			fmt.Printf("[lyrics] Embedded lyrics in FLAC: %s\n", filepath.Base(outPath))
		} else {
			fmt.Printf("[lyrics] No lyrics embedded in FLAC: %s\n", filepath.Base(outPath))
		}
	}

	if !cfg.ConvertKeepOriginal {
		if err := os.Remove(srcPath); err != nil {
			fmt.Println("Failed to remove original after conversion:", err)
		} else {
			track.SavePath = outPath
			track.SaveName = filepath.Base(outPath)
			fmt.Println("Original removed.")
		}
	} else {
		// Keep both but point track to new file (optional decision)
		track.SavePath = outPath
		track.SaveName = filepath.Base(outPath)
	}
}

// HasFlacLyrics checks whether a FLAC file has lyrics embedded in its metadata tags (LYRICS or UNSYNCEDLYRICS).
func HasFlacLyrics(ctx context.Context, flacPath string) bool {
	if flacPath == "" {
		return false
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "ffprobe",
		"-v", "error",
		"-show_entries", "format_tags=LYRICS:format_tags=lyrics:format_tags=UNSYNCEDLYRICS:format_tags=unsyncedlyrics",
		"-of", "default=noprint_wrappers=1:nokey=1",
		flacPath,
	)
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return len(strings.TrimSpace(string(out))) > 0
}

// EmbedFlacLyrics losslessly injects Vorbis LYRICS into an existing FLAC file in-place using ffmpeg stream copy.
func EmbedFlacLyrics(ffmpegPath, flacPath, lrc string) error {
	if ffmpegPath == "" {
		ffmpegPath = "ffmpeg"
	}
	tmpPath := flacPath + ".tmp.flac"
	args := []string{
		"-y",
		"-i", flacPath,
		"-map", "0",
		"-c", "copy",
		"-metadata", fmt.Sprintf("LYRICS=%s", normalizeLyrics(lrc)),
		tmpPath,
	}
	cmd := exec.Command(ffmpegPath, args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("ffmpeg stream-copy error: %v, out: %s", err, string(out))
	}
	if err := os.Rename(tmpPath, flacPath); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return nil
}

// CheckOrEmbedFlacLyrics checks whether an existing FLAC file already has lyrics embedded.
// If missing and lyrics are available with embed enabled, embeds them losslessly in-place.
// Logs whether lyrics were already embedded, newly embedded, or not embedded.
// Returns true if the FLAC file currently has embedded lyrics.
func CheckOrEmbedFlacLyrics(ctx context.Context, ffmpegPath string, flacPath string, lrc string, embedEnabled bool) bool {
	if !strings.EqualFold(filepath.Ext(flacPath), ".flac") {
		return false
	}
	base := filepath.Base(flacPath)
	if HasFlacLyrics(ctx, flacPath) {
		fmt.Printf("[lyrics] Lyrics already embedded in FLAC: %s\n", base)
		return true
	}
	if lrc != "" && embedEnabled {
		if err := EmbedFlacLyrics(ffmpegPath, flacPath, lrc); err != nil {
			fmt.Printf("[lyrics] Failed to embed lyrics in existing FLAC: %v\n", err)
			return false
		}
		fmt.Printf("[lyrics] Embedded lyrics in existing FLAC: %s\n", base)
		return true
	}
	fmt.Printf("[lyrics] No lyrics embedded in FLAC: %s\n", base)
	return false
}
