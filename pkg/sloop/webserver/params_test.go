/*
 * Copyright (c) 2019, salesforce.com, inc.
 * All rights reserved.
 * Licensed under the BSD 3-Clause license.
 * For full license text, see LICENSE.txt file in the repo root or
 * https://opensource.org/licenses/BSD-3-Clause
 */

package webserver

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/url"
	"testing"
	"time"
)

const (
	someParam = "someparam"
)

var (
	someDefaultTime       = time.Date(2019, 11, 24, 6, 5, 4, 3*1000*1000, time.UTC)
	someOtherTime         = someDefaultTime.Add(time.Minute)
	simeOtherTimeRoundSec = someOtherTime.Round(time.Second)
	simeOtherTimeRoundMs  = someOtherTime.Round(time.Millisecond)
	someDefaultDuration   = time.Minute
	someOtherDuration     = time.Hour
	someDefaultString     = "abc"
	someOtherString       = "def"
	someDirtyString       = "abcABC019-_. !@#$%^&*()+"
	dirtyStringCleaned    = "abcABC019-_."
)

func Test_timeFromParam_EmptyStrReturnDefault(t *testing.T) {
	request := &http.Request{}
	request.URL = &url.URL{}

	ret, err := timeFromUnixTimeParam(request, someParam, someDefaultTime, time.Millisecond)
	assert.Nil(t, err)
	assert.Equal(t, ret, someDefaultTime)
}

func Test_timeFromParam_InvalidStringReturnsError(t *testing.T) {
	request := &http.Request{}
	request.URL, _ = url.Parse(fmt.Sprintf("http://localhost/a?%v=%v", someParam, "thisIsNotANumber"))
	_, err := timeFromUnixTimeParam(request, someParam, someDefaultTime, time.Millisecond)
	assert.NotNil(t, err)
}

func Test_timeFromParam_UnixTimeWorksMillisecond(t *testing.T) {
	request := &http.Request{}
	request.URL, _ = url.Parse(fmt.Sprintf("http://localhost/a?%v=%v", someParam, someOtherTime.Unix()*1000+3))
	ret, err := timeFromUnixTimeParam(request, someParam, time.Time{}, time.Millisecond)
	assert.Nil(t, err)
	assert.Equal(t, simeOtherTimeRoundMs, ret)
}

func Test_timeFromParam_UnixTimeWorksSecond(t *testing.T) {
	request := &http.Request{}
	request.URL, _ = url.Parse(fmt.Sprintf("http://localhost/a?%v=%v", someParam, someOtherTime.Unix()))
	ret, err := timeFromUnixTimeParam(request, someParam, time.Time{}, time.Second)
	assert.Nil(t, err)
	assert.Equal(t, simeOtherTimeRoundSec, ret)
}

func Test_durationFromParam_EmptyReturnsDefault(t *testing.T) {
	request := &http.Request{}
	request.URL = &url.URL{}

	ret, err := durationFromParam(request, someParam, someDefaultDuration)
	assert.Nil(t, err)
	assert.Equal(t, ret, someDefaultDuration)
}

func Test_durationFromParam_InvalidDuration_ReturnsError(t *testing.T) {
	request := &http.Request{}
	request.URL, _ = url.Parse(fmt.Sprintf("http://localhost/a?%v=%v", someParam, "NotADuration"))
	_, err := durationFromParam(request, someParam, someDefaultDuration)
	assert.NotNil(t, err)
}

func Test_durationFromParam_GAoodDuration_ReturnsError(t *testing.T) {
	request := &http.Request{}
	request.URL, _ = url.Parse(fmt.Sprintf("http://localhost/a?%v=%v", someParam, someOtherDuration))
	ret, err := durationFromParam(request, someParam, someDefaultDuration)
	assert.Nil(t, err)
	assert.Equal(t, someOtherDuration, ret)
}

func Test_cleanStringFromParam_Empty_ReturnsDefault(t *testing.T) {
	request := &http.Request{}
	request.URL = &url.URL{}

	ret := cleanStringFromParam(request, someParam, someDefaultString)
	assert.Equal(t, someDefaultString, ret)
}

func Test_cleanStringFromParam_GoodIn_GoodOut(t *testing.T) {
	request := &http.Request{}

	request.URL, _ = url.Parse(fmt.Sprintf("http://localhost/a?%v=%v", someParam, someOtherString))
	ret := cleanStringFromParam(request, someParam, someDefaultString)
	assert.Equal(t, someOtherString, ret)
}

func Test_cleanStringFromParam_DirtyIn_CleanOut(t *testing.T) {
	request := &http.Request{}

	request.URL, _ = url.Parse(fmt.Sprintf("http://localhost/a?%v=%v", someParam, url.QueryEscape(someDirtyString)))
	ret := cleanStringFromParam(request, someParam, someDefaultString)
	assert.Equal(t, dirtyStringCleaned, ret)
}

func Test_numberFromParam_GoodCase(t *testing.T) {
	request := &http.Request{}
	request.URL, _ = url.Parse(fmt.Sprintf("http://localhost/a?%v=%v", someParam, "123"))
	ret := numberFromParam(request, someParam, 456)
	assert.Equal(t, 123, ret)
}

func Test_numberFromParam_ErrorReturnsDefault(t *testing.T) {
	request := &http.Request{}
	request.URL, _ = url.Parse(fmt.Sprintf("http://localhost/a?%v=%v", someParam, "abc"))
	ret := numberFromParam(request, someParam, 456)
	assert.Equal(t, 456, ret)
}

func Test_numberFromParam_EmptyReturnsDefault(t *testing.T) {
	request := &http.Request{}
	request.URL, _ = url.Parse(fmt.Sprintf("http://localhost/"))
	ret := numberFromParam(request, someParam, 456)
	assert.Equal(t, 456, ret)
}
