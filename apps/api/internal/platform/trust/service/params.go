package service

import "time"

const foldWindow = 30 * 24 * time.Hour

var callbackBackoff = []time.Duration{
	1 * time.Minute,
	5 * time.Minute,
	25 * time.Minute,
	2 * time.Hour,
	10 * time.Hour,
}

const callbackMaxAttempts = 5

const (
	maxScanTextRunes = 8000

	scanBatchSize = 20
	scanInterval  = 60 * time.Second

	maxScanAttempts = 3

	maxScanSampleRate = 0.05

	scanSamplePriority float32 = 0.05
)
