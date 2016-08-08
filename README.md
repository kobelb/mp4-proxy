# MP4 Proxy
Chrome 52 has a [bug](https://bugs.chromium.org/p/chromium/issues/detail?id=632624) where videos recorded vertically on mobile devices appear stretched when they are played back. The has been fixed in Chrome 53; however, it's not important enough of a bug for Chrome to hotfix it in 52. This project attempts to fix this issue in Chrome 52 by modifying the videos on the fly to display properly.

## Approach
The underlying issue is caused by the display matrix in the mp4's track header box. Whenever a video is rotated 90&#176; or 270&#176; Chrome uses the display matrix to rotate the video properly; however, it's also stretching the video by interpreting the track header box's width and height incorrectly. This solution switches the height and width in the track header box when it detects a problematic rotation so that the video displays correctly.

## Usage
This project has only been tested on Go 1.6 and assumes you have Go already installed.

#### Installation
`go get github.com/kobelb/mp4-proxy`

#### Build
`go build`

There's also a `build.sh` that will statically build the app for Linux, which is currently used for the Docker image. 
 
#### Run
`mp4-proxy` or `./mp4-proxy` depending on whether you installed the app after you built it. 

Request the video using a URL like below `http://localhost:5000?url=https%3A%2F%2Faddpipevideos.s3.amazonaws.com%2F29d9277c9b566c0e2322ce4540f36f5d%2Fmvs285869022.mp4`

By default the app listens for requests on port 5000, and it's using port 5001 for a [groupcache](https://github.com/golang/groupcache) that keeps all the dimensions for the videos cached. You can change the ports by setting the `PORT` and the `CACHE_PORT` environment variables.