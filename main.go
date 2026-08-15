package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// Cấu trúc dữ liệu đại diện cho 1 Video
type Video struct {
	Title        string
	Views        int64
	ThumbnailURL string
	VideoURL     string
	PublishedAt  string
	OutlierScore float64
}

func main() {
	myApp := app.New()
	myWindow := myApp.NewWindow("🚀 YouTube Outlier Finder (Go Edition)")
	myWindow.Resize(fyne.NewSize(1100, 700))

	// --- 1. HEADER & KHU VỰC TÌM KIẾM ---
	apiKeyEntry := widget.NewPasswordEntry()
	apiKeyEntry.SetPlaceHolder("Nhập YouTube API Key...")

	channelEntry := widget.NewEntry()
	channelEntry.SetPlaceHolder("Nhập tên kênh hoặc Handle (ví dụ: VietCetera, @MrBeast)...")

	// --- 2. SIDEBAR BỘ LỌC ---
	thresholdLabel := widget.NewLabel(" Ngưỡng Outlier: 3.0x")
	thresholdSlider := widget.NewSlider(1.5, 10.0)
	thresholdSlider.SetValue(3.0)
	thresholdSlider.Step = 0.5
	thresholdSlider.OnChanged = func(val float64) {
		thresholdLabel.SetText(fmt.Sprintf(" Ngưỡng Outlier: %.1fx", val))
	}

	statusLabel := widget.NewLabel("Trạng thái: Sẵn sàng")

	// --- 3. METRICS TỔNG QUAN ---
	medianMetric := widget.NewLabelWithStyle("—", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	outlierCountMetric := widget.NewLabelWithStyle("—", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	topScoreMetric := widget.NewLabelWithStyle("—", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})

	metricsBox := container.NewGridWithColumns(3,
		widget.NewCard("Lượt xem Trung vị", "", medianMetric),
		widget.NewCard("Số Outlier Tìm thấy", "", outlierCountMetric),
		widget.NewCard("Outlier Cao nhất", "", topScoreMetric),
	)

	// --- 4. KHU VỰC HIỂN THỊ DANH SÁCH VIDEO ---
	videoGrid := container.NewGridWithColumns(3)
	scrollableContent := container.NewVScroll(container.NewVBox(metricsBox, widget.NewSeparator(), videoGrid))

	// --- LOGIC XỬ LÝ KHI BẤM NÚT TÌM KIẾM ---
	searchBtn := widget.NewButton("Phân Tích Outliers 🔍", func() {
		apiKey := apiKeyEntry.Text
		channelQuery := channelEntry.Text

		if apiKey == "" || channelQuery == "" {
			statusLabel.SetText("⚠️ Vui lòng nhập đầy đủ API Key và Tên kênh!")
			return
		}

		statusLabel.SetText("⏳ Đang tải dữ liệu từ YouTube...")
		videoGrid.Objects = nil // Xóa danh sách cũ

		go func() {
			title, median, videos, err := fetchYouTubeData(apiKey, channelQuery, 30)
			if err != nil {
				statusLabel.SetText(fmt.Sprintf("❌ Lỗi: %v", err))
				return
			}

			// Lọc theo ngưỡng
			threshold := thresholdSlider.Value
			var outliers []Video
			maxScore := 0.0

			for _, v := range videos {
				if v.OutlierScore >= threshold {
					outliers = append(outliers, v)
				}
				if v.OutlierScore > maxScore {
					maxScore = v.OutlierScore
				}
			}

			// Sắp xếp giảm dần theo điểm Outlier
			sort.Slice(videos, func(i, j int) bool {
				return videos[i].OutlierScore > videos[j].OutlierScore
			})

			// Cập nhật Metrics
			medianMetric.SetText(fmt.Sprintf("%s views", formatNumber(median)))
			outlierCountMetric.SetText(fmt.Sprintf("%d video", len(outliers)))
			topScoreMetric.SetText(fmt.Sprintf("%.1fx", maxScore))

			// Tạo các thẻ Video (Cards)
			for _, v := range videos {
				vCopy := v
				var badgeText string
				if vCopy.OutlierScore >= threshold {
					badgeText = fmt.Sprintf("🔥 OUTLIER: %.1fx", vCopy.OutlierScore)
				} else {
					badgeText = fmt.Sprintf("⚪ Điểm: %.1fx", vCopy.OutlierScore)
				}

				badgeLabel := widget.NewLabelWithStyle(badgeText, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
				titleLabel := widget.NewLabel(vCopy.Title)
				titleLabel.Wrapping = fyne.TextWrapWord

				statsLabel := widget.NewLabel(fmt.Sprintf("👀 %s views | 📅 %s", formatNumber(vCopy.Views), vCopy.PublishedAt))
				statsLabel.Importance = widget.LowImportance

				// Tải ảnh Thumbnail
				img := canvas.NewImageFromURI(fyne.NewURI(vCopy.ThumbnailURL))
				img.SetMinSize(fyne.NewSize(240, 135))
				img.FillMode = canvas.ImageFillContain

				cardContent := container.NewVBox(img, badgeLabel, titleLabel, statsLabel)
				card := widget.NewCard("", "", cardContent)
				videoGrid.Add(card)
			}

			statusLabel.SetText(fmt.Sprintf("✅ Phân tích thành công kênh: %s", title))
			videoGrid.Refresh()
		}()
	})

	// Layout tổng thể
	searchBar := container.NewBorder(nil, nil, nil, searchBtn, channelEntry)
	header := container.NewVBox(
		widget.NewLabelWithStyle("🚀 YOUTUBE OUTLIER FINDER", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		container.NewGridWithColumns(2, widget.NewLabel("API Key:"), apiKeyEntry),
		searchBar,
		statusLabel,
	)

	sidebar := container.NewVBox(
		widget.NewLabelWithStyle("⚙️ BỘ LỌC", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		thresholdLabel,
		thresholdSlider,
	)

	mainLayout := container.NewBorder(header, nil, sidebar, nil, scrollableContent)
	myWindow.SetContent(mainLayout)
	myWindow.ShowAndRun()
}

// --- HÀM GỌI API YOUTUBE CHUẨN (KHÔNG DÙNG THƯ VIỆN NGOÀI) ---
func fetchYouTubeData(apiKey, query string, maxResults int) (string, int64, []Video, error) {
	// 1. Tìm ID Kênh
	searchURL := fmt.Sprintf("https://www.googleapis.com/youtube/v3/search?part=snippet&type=channel&q=%s&key=%s", url.QueryEscape(query), apiKey)
	resp, err := http.Get(searchURL)
	if err != nil {
		return "", 0, nil, err
	}
	defer resp.Body.Close()

	var searchData struct {
		Items []struct {
			ID struct{ ChannelID string } `json:"id"`
			Snippet struct{ Title string } `json:"snippet"`
		} `json:"items"`
	}
	json.NewDecoder(resp.Body).Decode(&searchData)
	if len(searchData.Items) == 0 {
		return "", 0, nil, fmt.Errorf("không tìm thấy kênh")
	}

	channelID := searchData.Items[0].ID.ChannelID
	channelTitle := searchData.Items[0].Snippet.Title

	// 2. Lấy Playlist Uploads
	channelURL := fmt.Sprintf("https://www.googleapis.com/youtube/v3/channels?part=contentDetails&id=%s&key=%s", channelID, apiKey)
	resp2, _ := http.Get(channelURL)
	defer resp2.Body.Close()

	var chData struct {
		Items []struct {
			ContentDetails struct {
				RelatedPlaylists struct{ Uploads string } `json:"relatedPlaylists"`
			} `json:"contentDetails"`
		} `json:"items"`
	}
	json.NewDecoder(resp2.Body).Decode(&chData)
	uploadsID := chData.Items[0].ContentDetails.RelatedPlaylists.Uploads

	// 3. Lấy Danh sách Video
	playlistURL := fmt.Sprintf("https://www.googleapis.com/youtube/v3/playlistItems?part=snippet&playlistId=%s&maxResults=%d&key=%s", uploadsID, maxResults, apiKey)
	resp3, _ := http.Get(playlistURL)
	defer resp3.Body.Close()

	var plData struct {
		Items []struct {
			Snippet struct {
				ResourceId struct{ VideoID string } `json:"resourceId"`
			} `json:"snippet"`
		} `json:"items"`
	}
	json.NewDecoder(resp3.Body).Decode(&plData)

	var videoIDs []string
	for _, item := range plData.Items {
		videoIDs = append(videoIDs, item.Snippet.ResourceId.VideoID)
	}

	// 4. Lấy Thống kê Chi tiết từng Video
	videosURL := fmt.Sprintf("https://www.googleapis.com/youtube/v3/videos?part=snippet,statistics&id=%s&key=%s", strings.Join(videoIDs, ","), apiKey)
	resp4, _ := http.Get(videosURL)
	defer resp4.Body.Close()

	var vData struct {
		Items []struct {
			ID      string `json:"id"`
			Snippet struct {
				Title        string `json:"title"`
				PublishedAt  string `json:"publishedAt"`
				Thumbnails   struct {
					Medium struct{ URL string } `json:"medium"`
				} `json:"thumbnails"`
			} `json:"snippet"`
			Statistics struct {
				ViewCount string `json:"viewCount"`
			} `json:"statistics"`
		} `json:"items"`
	}
	json.NewDecoder(resp4.Body).Decode(&vData)

	var videos []Video
	var viewsList []int64

	for _, item := range vData.Items {
		views, _ := strconv.ParseInt(item.Statistics.ViewCount, 10, 64)
		viewsList = append(viewsList, views)

		pubDate := item.Snippet.PublishedAt
		if len(pubDate) >= 10 {
			pubDate = pubDate[:10]
		}

		videos = append(videos, Video{
			Title:        item.Snippet.Title,
			Views:        views,
			ThumbnailURL: item.Snippet.Thumbnails.Medium.URL,
			VideoURL:     "https://www.youtube.com/watch?v=" + item.ID,
			PublishedAt:  pubDate,
		})
	}

	// Tính Lượt xem Trung vị (Median)
	median := calculateMedian(viewsList)

	// Tính Outlier Score
	for i := range videos {
		if median > 0 {
			videos[i].OutlierScore = float64(videos[i].Views) / float64(median)
		}
	}

	return channelTitle, median, videos, nil
}

func calculateMedian(numbers []int64) int64 {
	if len(numbers) == 0 {
		return 1
	}
	sort.Slice(numbers, func(i, j int) bool { return numbers[i] < numbers[j] })
	mid := len(numbers) / 2
	if len(numbers)%2 == 0 {
		return (numbers[mid-1] + numbers[mid]) / 2
	}
	return numbers[mid]
}

func formatNumber(n int64) string {
	in := strconv.FormatInt(n, 10)
	out := ""
	for i, c := range in {
		if i > 0 && (len(in)-i)%3 == 0 {
			out += ","
		}
		out += string(c)
	}
	return out
}
