// The MIT License (MIT)
//
// Copyright (c) 2014 winlin
//
// Permission is hereby granted, free of charge, to any person obtaining a copy of
// this software and associated documentation files (the "Software"), to deal in
// the Software without restriction, including without limitation the rights to
// use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of
// the Software, and to permit persons to whom the Software is furnished to do so,
// subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS
// FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR
// COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER
// IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN
// CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.

package rtmp

import (
	"net"
	"net/url"
	"strings"
	"strconv"
	"fmt"
)

const (
	CodecAMF0 = 0
	CodecAMF3 = 3
	DefaultPort = 1935
	// 5.6. Set Peer Bandwidth (6)
	// the Limit type field:
	// hard (0), soft (1), or dynamic (2)
	PeerBandwidthHard = 0
	PeerBandwidthSoft = 1
	PeerBandwidthDynamic = 2
)

/**
* the signature for packets to client.
*/
const SIG_FMS_VER = "3,5,3,888"
const SIG_AMF0_VER = 0
const SIG_CLIENT_ID = "ASAICiss"

/**
* onStatus consts.
*/
const SLEVEL = "level"
const SCODE = "code"
const SDESC = "description"
const SDETAILS = "details"
const SCLIENT_ID = "clientid"
// status value
const SLEVEL_Status = "status"
// status error
const SLEVEL_Error = "error"
// code value
const SCODE_ConnectSuccess = "NetConnection.Connect.Success"
const SCODE_ConnectRejected = "NetConnection.Connect.Rejected"
const SCODE_StreamReset = "NetStream.Play.Reset"
const SCODE_StreamStart = "NetStream.Play.Start"
const SCODE_StreamPause = "NetStream.Pause.Notify"
const SCODE_StreamUnpause = "NetStream.Unpause.Notify"
const SCODE_PublishStart = "NetStream.Publish.Start"
const SCODE_DataStart = "NetStream.Data.Start"
const SCODE_UnpublishSuccess = "NetStream.Unpublish.Success"

// FMLE
const AMF0_COMMAND_ON_FC_PUBLISH = "onFCPublish"
const AMF0_COMMAND_ON_FC_UNPUBLISH = "onFCUnpublish"

// type of client
const (
	CLIENT_TYPE_Unknown = iota
	CLIENT_TYPE_Play
	CLIENT_TYPE_FMLEPublish
	CLIENT_TYPE_FlashPublish
)

/**
* the original request from client.
*/
// @see: SrsRequest
type Request struct {
	/**
	* tcUrl: rtmp://request_vhost:port/app/stream
	* support pass vhost in query string, such as:
	*	rtmp://ip:port/app?vhost=request_vhost/stream
	*	rtmp://ip:port/app...vhost...request_vhost/stream
	*/
	TcUrl string
	PageUrl string
	SwfUrl string
	// enum CodecAMF0 or CodecAMF3
	ObjectEncoding int

	/**
	* parsed uri info from TcUrl and stream.
	 */
	Schema string
	Vhost string
	Port string
	App string
	Stream string
}
func NewRequest() (*Request) {
	r := &Request{}
	r.ObjectEncoding = CodecAMF0
	r.Port = strconv.Itoa(DefaultPort)
	return r
}

func (r *Request) discovery_app() (err error) {
	// parse ...vhost... to ?vhost=
	var v string = r.TcUrl
	if !strings.Contains(v, "?") {
		v = strings.Replace(v, "...", "?", 1)
		v = strings.Replace(v, "...", "=", 1)
	}
	for strings.Contains(v, "...") {
		v = strings.Replace(v, "...", "&", 1)
		v = strings.Replace(v, "...", "=", 1)
	}
	r.TcUrl = v

	// parse standard rtmp url.
	var u *url.URL
	if u, err = url.Parse(r.TcUrl); err != nil {
		return
	}

	r.Schema, r.App = u.Scheme, u.Path

	r.Vhost = u.Host
	if strings.Contains(u.Host, ":") {
		host_parts := strings.Split(u.Host, ":")
		r.Vhost, r.Port = host_parts[0], host_parts[1]
	}

	// discovery vhost from query.
	query := u.Query()
	for k, _ := range query {
		if strings.ToLower(k) == "vhost" && query.Get(k) != "" {
			r.Vhost = query.Get(k)
		}
	}

	// resolve the vhost from config
	// TODO: FIXME: implements it
	// TODO: discovery the params of vhost.

	if r.Schema = strings.Trim(r.Schema, "/\n\r "); r.Schema == ""{
		return RtmpError{code:ERROR_RTMP_REQ_TCURL, desc:fmt.Sprintf("discovery schema failed. tcUrl=%v", r.TcUrl)}
	}
	if r.Vhost = strings.Trim(r.Vhost, "/\n\r "); r.Vhost == "" {
		return RtmpError{code:ERROR_RTMP_REQ_TCURL, desc:fmt.Sprintf("discovery vhost failed. tcUrl=%v", r.TcUrl)}
	}
	if r.App = strings.Trim(r.App, "/\n\r "); r.App == "" {
		return RtmpError{code:ERROR_RTMP_REQ_TCURL, desc:fmt.Sprintf("discovery app failed. tcUrl=%v", r.TcUrl)}
	}
	if r.Port = strings.Trim(r.Port, "/\n\r "); r.Port == "" {
		return RtmpError{code:ERROR_RTMP_REQ_TCURL, desc:fmt.Sprintf("discovery port failed. tcUrl=%v", r.TcUrl)}
	}

	return
}

/**
* genereate the stream id, to response CreateStream() request.
 */
type RtmpStreamIdGenerator interface {
	StreamId() (n int)
}

/**
* the rtmp server interface, user can create it by func NewServer().
 */
type Server interface {
	/**
	* handshake with client, try complex handshake first, use simple if failed.
	 */
	Handshake() (err error)
	/**
	* expect client send the connect app request,
	* @param req set and parse data to the request
	 */
	ConnectApp(req *Request) (err error)
	/**
	* set the ack size window
	* @param ack_size in bytes, for example, 2.5 * 1000 * 1000
	 */
	SetWindowAckSize(ack_size uint32) (err error)
	/**
	* set the peer bandwidth,
	* @param bandwidth in bytes, for example, 2.5 * 1000 * 1000
	* @param bw_type can be PeerBandwidthHard, PeerBandwidthSoft or PeerBandwidthDynamic
	 */
	SetPeerBandwidth(bandwidth uint32, bw_type byte) (err error)
	/**
	* response the client connect app request
	* @param req the request data genereated by ConnectApp
	* @param server_ip the ip of server to send to client, ignore if "".
	* 		for example, "192.168.1.12"
	* @param extra_data the extra data to send to client, ignore if nil.
	* 		where the slice used to keep the sequence of data.
	* 		for example, []map[string]string { {"server":"go.srs"}, {"version":"1.0"} }
	 */
	ReponseConnectApp(req *Request, server_ip string, extra_data []map[string]string) (err error)
	/**
	* call client onBWDone() method
	 */
	CallOnBWDone() (err error)
	/**
	* identify the client stream type, can be const value:
	* 		CLIENT_TYPE_Unknown, cannot identify the client.
	* 		CLIENT_TYPE_Play the client is a play client, for example, the Flash play.
	* 		CLIENT_TYPE_FMLEPublish the client is publish client use FMLE schema, for example, the adobe FMLE
	* 		CLIENT_TYPE_FlashPublish the client is publish client use Flash schema, for example, the Flash publish.
	 */
	IdentifyClient(stream_id_generator RtmpStreamIdGenerator) (client_type int, stream_name string, err error)
}
func NewServer(conn *net.TCPConn) (Server, error) {
	var err error
	r := &server{}
	if r.protocol, err = NewProtocol(conn); err != nil {
		return r, err
	}
	return r, err
}

type server struct {
	protocol Protocol
}

func (r *server) Handshake() (err error) {
	// TODO: FIXME: try complex then simple handshake.
	err = r.protocol.SimpleHandshake2Client()
	return
}

func (r *server) ConnectApp(req *Request) (err error) {
	//var msg *Message
	var pkt *ConnectAppPacket
	if _, err = r.protocol.ExpectMessage(&pkt); err != nil {
		return
	}

	var ok bool
	if req.TcUrl, ok = pkt.CommandObject.GetPropertyString("tcUrl"); !ok {
		err = RtmpError{code:ERROR_RTMP_REQ_CONNECT, desc:"invalid request, must specifies the tcUrl."}
		return
	}
	if v, ok := pkt.CommandObject.GetPropertyString("pageUrl"); ok {
		req.PageUrl = v
	}
	if v, ok := pkt.CommandObject.GetPropertyString("swfUrl"); ok {
		req.SwfUrl = v
	}
	if v, ok := pkt.CommandObject.GetPropertyNumber("objectEncoding"); ok {
		req.ObjectEncoding = int(v)
	}

	return req.discovery_app()
}

func (r *server) SetWindowAckSize(ack_size uint32) (err error) {
	pkt := SetWindowAckSizePacket{AcknowledgementWindowSize:ack_size}
	return r.protocol.SendPacket(&pkt, uint32(0))
}

func (r *server) SetPeerBandwidth(bandwidth uint32, bw_type byte) (err error) {
	pkt := SetPeerBandwidthPacket{Bandwidth:bandwidth, BandwidthType:bw_type}
	return r.protocol.SendPacket(&pkt, uint32(0))
}

func (r *server) ReponseConnectApp(req *Request, server_ip string, extra_data []map[string]string) (err error) {
	data := NewAmf0EcmaArray()
	data.Set("version", ToAmf0(SIG_FMS_VER))
	if server_ip != "" {
		data.Set("srs_server_ip", ToAmf0(server_ip))
	}
	for _, item := range extra_data {
		for k, v := range item {
			data.Set(k, ToAmf0(v))
		}
	}

	var pkt *ConnectAppResPacket = NewConnectAppResPacket()
	pkt.PropsSet("fmsVer", "FMS/"+SIG_FMS_VER).PropsSet("capabilities", float64(127)).PropsSet("mode", float64(1))
	pkt.InfoSet(SLEVEL, SLEVEL_Status).InfoSet(SCODE, SCODE_ConnectSuccess).InfoSet(SDESC, "Connection succeeded")
	pkt.InfoSet("objectEncoding", float64(req.ObjectEncoding)).InfoSet("data", data)

	return r.protocol.SendPacket(pkt, uint32(0))
}

func (r *server) CallOnBWDone() (err error) {
	var pkt *OnBWDonePacket = NewOnBWDonePacket()
	return r.protocol.SendPacket(pkt, uint32(0))
}

func (r *server) IdentifyClient(stream_id_generator RtmpStreamIdGenerator) (client_type int, stream_name string, err error) {
	client_type = CLIENT_TYPE_Unknown
	for {
		var msg *Message
		if msg, err = r.protocol.RecvMessage(); err != nil {
			return
		}

		if !msg.Header.IsAmf0Command() && !msg.Header.IsAmf3Command() {
			continue
		}

		var pkt interface {}
		if pkt, err = r.protocol.DecodeMessage(msg); err != nil {
			return
		}

		if pkt, ok := pkt.(*CreateStreamPacket); ok {
			return r.identify_create_stream_client(pkt, stream_id_generator)
		}
	}
	return
}
func (r *server) identify_create_stream_client(req *CreateStreamPacket, stream_id_generator RtmpStreamIdGenerator) (client_type int, stream_name string, err error) {
	pkt := NewCreateStreamResPacket(req.TransactionId, float64(stream_id_generator.StreamId()))
	if err = r.protocol.SendPacket(pkt, uint32(0)); err != nil {
		return
	}

	for {
		var msg *Message
		if msg, err = r.protocol.RecvMessage(); err != nil {
			return
		}

		if !msg.Header.IsAmf0Command() && !msg.Header.IsAmf3Command() {
			continue
		}

		var pkt interface {}
		if pkt, err = r.protocol.DecodeMessage(msg); err != nil {
			return
		}

		if pkt, ok := pkt.(*PlayPacket); ok {
			return r.identify_play_client(pkt)
		}
		if pkt, ok := pkt.(*PublishPacket); ok {
			return r.identify_flash_publish_client(pkt)
		}
	}
	return
}
func (r *server) identify_play_client(req *PlayPacket) (client_type int, stream_name string, err error) {
	return
}
func (r *server) identify_flash_publish_client(req *PublishPacket) (client_type int, stream_name string, err error) {
	return
}
