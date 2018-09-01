package main

import (
	"image"
	"image/color"
	"log"
	"math"
	"time"

	"gocv.io/x/gocv"
)

func trackObject(videoCap *gocv.VideoCapture, window *gocv.Window) {
	// Color to mark the contours
	blue := color.RGBA{0, 0, 255, 0}

	// Lower and upper bound HSV color to track
	lowerBound := gocv.NewScalar(50.0, 80.0, 40.0, 0.0)
	upperBound := gocv.NewScalar(85.0, 255.0, 255.0, 0.0)

	mask := gocv.NewMat()
	img := gocv.NewMat()
	hsvImg := gocv.NewMat()
	kernelOpen := gocv.GetStructuringElement(gocv.MorphRect, image.Pt(5, 5))
	kernelClose := gocv.GetStructuringElement(gocv.MorphRect, image.Pt(20, 20))
	frameSkipRemainder := float64(0)
	for {
		timeStart := time.Now()
		// read next image
		if ok := videoCap.Read(&img); !ok {
			log.Println("cannot read video file")
			continue
		}
		if img.Empty() {
			log.Println("skipping frame")
			continue
		}

		gocv.CvtColor(img, &hsvImg, gocv.ColorBGRToHSV)
		gocv.InRangeWithScalar(hsvImg, lowerBound, upperBound, &mask)

		gocv.MorphologyEx(mask, &mask, gocv.MorphOpen, kernelOpen)
		gocv.MorphologyEx(mask, &mask, gocv.MorphClose, kernelClose)

		conts := gocv.FindContours(mask, gocv.RetrievalExternal, gocv.ChainApproxNone)
		if len(conts) > 0 {
			// Only keep the largest contour
			largestCont := 0
			largestContCount := 0
			for i := range conts {
				length := len(conts[i])
				if length > largestContCount {
					largestCont = i
					largestContCount = length
				}
			}
			conts = conts[largestCont : largestCont+1]

			rect := gocv.MinAreaRect(conts[0])

			// Steer drone
			offX := (mask.Cols() / 2) - rect.Center.X
			offY := (mask.Rows() / 2) - rect.Center.Y

			// If offX > 0, we must steer left
			if offX > 10 {
				globalController.SetDirectionX(-1)
			} else if offX < -10 {
				globalController.SetDirectionX(1)
			} else {
				globalController.SetDirectionX(0)
			}

			// If offY > 0, we must go up
			if offY > 10 {
				globalController.SetDirectionY(1)
			} else if offY < -10 {
				globalController.SetDirectionY(-1)
			} else {
				globalController.SetDirectionY(0)
			}

			objectSize := (rect.Width + rect.Height) / 2

			if objectSize > 280 {
				// Object too large, go back
				globalController.SetDirectionZ(-1)
			} else if objectSize > 220 {
				// Object too small, go forward
				globalController.SetDirectionZ(1)
			} else {
				globalController.SetDirectionZ(0)
			}

			// Draw onto image
			gocv.DrawContours(&img, [][]image.Point{rect.Contour}, -1, blue, 3)
		} else {
			// Drone should stand still
			globalController.SetDirectionX(0)
			globalController.SetDirectionY(0)
			globalController.SetDirectionZ(0)
		}

		window.IMShow(img)
		window.WaitKey(1)
		skipFrames(videoCap, timeStart, &frameSkipRemainder)
	}
}

func showVideo(videoCap *gocv.VideoCapture, window *gocv.Window) {
	img := gocv.NewMat()
	frameSkipRemainder := float64(0)
	for {
		timeStart := time.Now()
		// read next image
		if ok := videoCap.Read(&img); !ok {
			log.Println("cannot read video file")
			continue
		}
		if img.Empty() {
			log.Println("skipping frame")
			continue
		}
		window.IMShow(img)
		window.WaitKey(1)
		skipFrames(videoCap, timeStart, &frameSkipRemainder)
	}
}

func trackOpticalFlow(videoCap *gocv.VideoCapture, window *gocv.Window) {
	// // color for the flow lines
	// blue := color.RGBA{0, 0, 255, 0}

	// // Size of flow lines
	// flowSize := float32(1.0)

	// prepare old image matrix
	oldImage := gocv.NewMat()
	defer oldImage.Close()
	oldImageGrey := gocv.NewMat()
	defer oldImageGrey.Close()

	// read first image
	for {
		if ok := videoCap.Read(&oldImage); !ok {
			log.Println("Could not read first image")
			return
		}
		if !oldImage.Empty() {
			gocv.CvtColor(oldImage, &oldImageGrey, gocv.ColorBGRToGray)
			break
		}
	}
	goodFeatures := gocv.NewMat()
	defer goodFeatures.Close()
	nextImage := gocv.NewMat()
	defer nextImage.Close()
	nextImageGrey := gocv.NewMat()
	defer nextImageGrey.Close()
	flow := gocv.NewMat()
	defer flow.Close()
	status := gocv.NewMat()
	defer status.Close()
	errors := gocv.NewMat()
	defer errors.Close()
	trackIterations := 20
	adjustIterations := 0
	frameSkipRemainder := float64(0)
	// read next pictures
	for {
		if trackIterations >= 20 {
			// Get features
			gocv.GoodFeaturesToTrack(oldImageGrey, &goodFeatures, 200, 0.01, 10)
			trackIterations = 0
		}
		timeStart := time.Now()
		// read next image
		if ok := videoCap.Read(&nextImage); !ok {
			log.Println("Could not read next image")
			return
		}
		if nextImage.Empty() {
			continue
		}
		gocv.CvtColor(nextImage, &nextImageGrey, gocv.ColorBGRToGray)

		// track optical flow
		gocv.CalcOpticalFlowPyrLK(oldImageGrey, nextImageGrey, goodFeatures, flow, &status, &errors)
		//gocv.CalcOpticalFlowFarneback(oldImageGrey, nextImageGrey, &flow, 0.5, 3, 15, 3, 5, 1.2, 0)

		// assign old grey
		nextImageGrey.CopyTo(&oldImageGrey)

		// goodFeatures will only have one col
		xSum := float32(0)
		ySum := float32(0)
		for row := 0; row < goodFeatures.Rows(); row++ {
			xSum = xSum + (goodFeatures.GetVecfAt(row, 0)[0] - flow.GetVecfAt(row, 0)[0])
			ySum = ySum + (goodFeatures.GetVecfAt(row, 0)[1] - flow.GetVecfAt(row, 0)[1])
			//log.Printf("0: %v\t1: %v", , )
		}
		xSum = xSum / float32(goodFeatures.Rows())
		ySum = ySum / float32(goodFeatures.Rows())
		log.Printf("x: %v\ty: %v", xSum, ySum)

		// Adjust left right
		if xSum < -10 && adjustIterations > 4 {
			// Drifting to the left
			log.Println("Adjusting steering to right!")
			adjustSteering(144, 16, 17)
			adjustIterations = 0
		} else if xSum > 10 && adjustIterations > 4 {
			// Drifting to the right
			log.Println("Adjusting steering! to left")
			adjustSteering(144, 16, 15)
			adjustIterations = 0
		}

		// TODO: Adjust forward backward

		// create flow image
		// by y += 5, x += 5 you can specify the grid
		// for y := 0; y < nextImage.Rows(); y += 10 {
		// 	for x := 0; x < nextImage.Cols(); x += 10 {
		// 		vec := flow.GetVecfAt(y, x)
		// 		gocv.Line(&nextImage, image.Point{x, y}, image.Point{int(x + int(vec[0]*flowSize)), int(y + int(vec[1]*flowSize))}, blue, 1)
		// 		gocv.Circle(&nextImage, image.Point{x, y}, 0, blue, 2)
		// 	}
		// }

		flow.CopyTo(&goodFeatures)

		window.IMShow(nextImage)
		window.WaitKey(1)
		skipFrames(videoCap, timeStart, &frameSkipRemainder)
		trackIterations = trackIterations + 1
		adjustIterations = adjustIterations + 1
	}
}

func skipFrames(videoCap *gocv.VideoCapture, timeStart time.Time, globalFrameSkipRemainder *float64) {
	// Showing frame usually takes longer than 40ms (25 FPS). We need to skip some frames
	framesToSkip := float64(time.Since(timeStart)-frameTime)/float64(frameTime) + *globalFrameSkipRemainder
	log.Println("framesToSkip: ", framesToSkip)
	if framesToSkip < 0 {
		// Frame was rendered too fast. Wait for the apropriate time before rendering the next one
		time.Sleep(time.Duration(float64(frameTime) * (-1 * framesToSkip)))
	} else {
		frameSkipInt, frameSkipRemainder := math.Modf(framesToSkip)
		*globalFrameSkipRemainder = frameSkipRemainder
		skip := int(frameSkipInt)
		if skip > 0 {
			videoCap.Grab(skip)
		}
	}
}
