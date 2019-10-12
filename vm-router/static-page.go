package main

import (
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
)

func handleRoot(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "https://blog.benjojo.co.uk/post/you-cant-curl-under-pressure", http.StatusTemporaryRedirect)
}

var validateSessionID = regexp.MustCompilePOSIX(`^[0-9a-f]{16}$`)

func handleReplay(w http.ResponseWriter, r *http.Request) {
	session := r.URL.Query().Get("s")
	if !validateSessionID.MatchString(session) {
		http.Error(w, "Invalid session ID", http.StatusNotFound)
		return
	}

	b, err := ioutil.ReadFile(fmt.Sprintf("./rec_%s.ttyrec", session))
	if err != nil {
		http.Error(w, "Unable to find that session", http.StatusNotFound)
		return
	}

	recordingBase64 := base64.StdEncoding.EncodeToString(b)
	w.Write([]byte(fmt.Sprintf(replayPage, recordingBase64)))
}

var replayPage = `
<!DOCTYPE html>
<html>
<head>
<meta content="origin" name="referrer">
<meta name="viewport" content="width=device-width, initial-scale=1">
</head>
<center>
<body style="background-color: #FFF">
	<p><div id="terminal-boot"></div>
	<button id="play-boot">Play</button></p>
	<script>var bootrecording = "%s";</script>

	<script src="https://blog.benjojo.co.uk/asset/8AqfZMzFPs"></script>
	<style type="text/css">
	/**
	 * Copyright (c) 2014 The xterm.js authors. All rights reserved.
	 * Copyright (c) 2012-2013, Christopher Jeffrey (MIT License)
	 * https://github.com/chjj/term.js
	 * @license MIT
	 *
	 * Permission is hereby granted, free of charge, to any person obtaining a copy
	 * of this software and associated documentation files (the "Software"), to deal
	 * in the Software without restriction, including without limitation the rights
	 * to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
	 * copies of the Software, and to permit persons to whom the Software is
	 * furnished to do so, subject to the following conditions:
	 *
	 * The above copyright notice and this permission notice shall be included in
	 * all copies or substantial portions of the Software.
	 *
	 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
	 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
	 * FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
	 * AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
	 * LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
	 * OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
	 * THE SOFTWARE.
	 *
	 * Originally forked from (with the author's permission):
	 *   Fabrice Bellard's javascript vt100 for jslinux:
	 *   http://bellard.org/jslinux/
	 *   Copyright (c) 2011 Fabrice Bellard
	 *   The original design remains. The terminal itself
	 *   has been extended to include xterm CSI codes, among
	 *   other features.
	 */
	
	/**
	 *  Default styles for xterm.js
	 */
	
	.xterm {
		font-feature-settings: "liga" 0;
		position: relative;
		user-select: none;
		-ms-user-select: none;
		-webkit-user-select: none;
	}
	
	.xterm.focus,
	.xterm:focus {
		outline: none;
	}
	
	.xterm .xterm-helpers {
		position: absolute;
		top: 0;
		/**
		 * The z-index of the helpers must be higher than the canvases in order for
		 * IMEs to appear on top.
		 */
		z-index: 10;
	}
	
	.xterm .xterm-helper-textarea {
		/*
		 * HACK: to fix IE's blinking cursor
		 * Move textarea out of the screen to the far left, so that the cursor is not visible.
		 */
		position: absolute;
		opacity: 0;
		left: -9999em;
		top: 0;
		width: 0;
		height: 0;
		z-index: -10;
		/** Prevent wrapping so the IME appears against the textarea at the correct position */
		white-space: nowrap;
		overflow: hidden;
		resize: none;
	}
	
	.xterm .composition-view {
		/* TODO: Composition position got messed up somewhere */
		background: #000;
		color: #FFF;
		display: none;
		position: absolute;
		white-space: nowrap;
		z-index: 1;
	}
	
	.xterm .composition-view.active {
		display: block;
	}
	
	.xterm .xterm-viewport {
		/* On OS X this is required in order for the scroll bar to appear fully opaque */
		background-color: #000;
		overflow-y: scroll;
		cursor: default;
		position: absolute;
		right: 0;
		left: 0;
		top: 0;
		bottom: 0;
	}
	
	.xterm .xterm-screen {
		position: relative;
	}
	
	.xterm .xterm-screen canvas {
		position: absolute;
		left: 0;
		top: 0;
	}
	
	.xterm .xterm-scroll-area {
		visibility: hidden;
	}
	
	.xterm-char-measure-element {
		display: inline-block;
		visibility: hidden;
		position: absolute;
		top: 0;
		left: -9999em;
		line-height: normal;
	}
	
	.xterm {
		cursor: text;
	}
	
	.xterm.enable-mouse-events {
		/* When mouse events are enabled (eg. tmux), revert to the standard pointer cursor */
		cursor: default;
	}
	
	.xterm.xterm-cursor-pointer {
		cursor: pointer;
	}
	
	.xterm.column-select.focus {
		/* Column selection mode */
		cursor: crosshair;
	}
	
	.xterm .xterm-accessibility,
	.xterm .xterm-message {
		position: absolute;
		left: 0;
		top: 0;
		bottom: 0;
		right: 0;
		z-index: 100;
		color: transparent;
	}
	
	.xterm .live-region {
		position: absolute;
		left: -9999px;
		width: 1px;
		height: 1px;
		overflow: hidden;
	}
	
	.xterm-dim {
		opacity: 0.5;
	}
	
	.xterm-underline {
		text-decoration: underline;
	} 
	</style>
	<script>
		function proposeGeometry(term) {
			if (!term.element.parentElement) {
				return null;
			}
			var parentElementStyle = window.getComputedStyle(term.element.parentElement);
			var parentElementHeight = parseInt(parentElementStyle.getPropertyValue('height'));
			var parentElementWidth = Math.max(0, parseInt(parentElementStyle.getPropertyValue('width')));
			var elementStyle = window.getComputedStyle(term.element);
			var elementPadding = {
				top: parseInt(elementStyle.getPropertyValue('padding-top')),
				bottom: parseInt(elementStyle.getPropertyValue('padding-bottom')),
				right: parseInt(elementStyle.getPropertyValue('padding-right')),
				left: parseInt(elementStyle.getPropertyValue('padding-left'))
			};
			var elementPaddingVer = elementPadding.top + elementPadding.bottom;
			var elementPaddingHor = elementPadding.right + elementPadding.left;
			var availableHeight = parentElementHeight - elementPaddingVer;
			var availableWidth = parentElementWidth - elementPaddingHor - term._core.viewport.scrollBarWidth;
			var geometry = {
				cols: Math.floor(availableWidth / term._core._renderCoordinator.dimensions.actualCellWidth),
				rows: Math.floor(availableHeight / term._core._renderCoordinator.dimensions.actualCellHeight)
			};
			return geometry;
		}
	
		function fit(term) {
			var geometry = proposeGeometry(term);
			if (geometry) {
				if (term.rows !== geometry.rows || term.cols !== geometry.cols) {
					term._core._renderCoordinator.clear();
					term.resize(geometry.cols, geometry.rows);
				}
			}
		}
	
		function apply(terminalConstructor) {
			terminalConstructor.prototype.proposeGeometry = function () {
				return proposeGeometry(this);
			};
			terminalConstructor.prototype.fit = function () {
				fit(this);
			};
		}
		//# sourceMappingURL=fit.js.map
	</script>
	<script>
	
			var bootterm = new Terminal();
			bootterm.open(document.getElementById('terminal-boot'));
			fit(bootterm);
	
			window.onresize = function(){
				fit(bootterm);
			};
	
			function convertB64ToBinary(base64) {
				var raw = window.atob(base64);
				var rawLength = raw.length;
				var array = new Uint8Array(new ArrayBuffer(rawLength));
	
				for(i = 0; i < rawLength; i++) {
					array[i] = raw.charCodeAt(i);
				}
				return array;
			}
	
			function playTTYplay(name,data,term) {
				array = convertB64ToBinary(data);
				window[name+"offset"] = 0;
				window[name+"lsec"] = 0;
				window[name+"lmsec"] = 0;
	
				setTimeout(blurTTY,1,name,array,term)
			}
	
			function blurTTY(name,data,term) {
				offset = window[name+"offset"];
	
				header = readTtyHeader(data,offset)
	
				datalength = header.len
	
				secdiff = header.secs - window[name+"lsec"];
				mdiff = header.msec - window[name+"lmsec"];
	
				window[name+"lsec"] = header.secs
				window[name+"lmsec"] = header.msec
	
				if (secdiff < 0) {
					secdiff = 0;
				}
	
				tdiff = secdiff * 1000;
				tdiff = tdiff + (mdiff/1000)
	
				if (tdiff > 1000) {
					tdiff = 1000;
				}
	
				stringToWrite = data.buffer.slice(offset+12, offset+12+datalength);
				var enc = new TextDecoder("utf-8");
	
				term.write(enc.decode(stringToWrite))
				window[name+"offset"] = offset+12+datalength;
	
				if (secdiff != 0) {
					setTimeout(blurTTY,1000,name,data,term)            
				} else {
					setTimeout(blurTTY,tdiff,name,data,term)            
				}
			}
	
			function readTtyHeader(data,offset) {
				// Should always return a len for how much string to send
				var u32bytes = data.buffer.slice(offset,offset+4);
				var secs = new Uint32Array(u32bytes)[0]
	
				var u32bytes = data.buffer.slice(offset+4,offset+8);
				var msec = new Uint32Array(u32bytes)[0]
	
				var u32bytes = data.buffer.slice(offset+8,offset+12);
				var len = new Uint32Array(u32bytes)[0]
				return {"len":len,"msec":msec,"secs":secs};
			}
	
			triggerboot = document.getElementById("play-boot");
			triggerboot.onclick = function(){
				playTTYplay("boot",bootrecording,bootterm)
			};
	
	
</script>
</div>
</body>
</html>	
`
