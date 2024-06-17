extends "ws_webrtc_client.gd"

var remote_player_data := Protobuff.Players.new()
var local_player_bytes : PackedByteArray

var puppet_data_bytes : PackedByteArray
var puppet_data : Protobuff.Players

signal player_id_received(player_id: int)
signal player_new_position(new_position : Vector3)
signal puppet_new_position(new_position : Vector3, id : int)
signal new_puppet(new_position : Vector3, player_data : Protobuff.Player)

var player_id := 0

const NOUNS := ["Computer", "Mountain", "Ocean", "Book", "Music", "Television", "Apple", "City", "Car", "Space"];
const ADJECTIVES := ["Happy", "Sad", "Excited", "Angry", "Joyful", "Peaceful", "Grumpy", "Elated", "Nervous", "Relaxed"];

func generate_name() -> StringName:
	return ADJECTIVES.pick_random() + " " + NOUNS.pick_random()	


var rtc_mp := WebRTCMultiplayerPeer.new()
var sealed := false

func _init() -> void:
	connected.connect(_connected)
	disconnected.connect(_disconnected)

	offer_received.connect(_offer_received)
	answer_received.connect(_answer_received)
	candidate_received.connect(_candidate_received)

	lobby_joined.connect(_lobby_joined)
	lobby_sealed.connect(_lobby_sealed)
	peer_connected.connect(_peer_connected)
	peer_disconnected.connect(_peer_disconnected)



func start(url: String = "ws://localhost:8080/ws", _lobby: String = "", _mesh: bool = true) -> void:
	stop()
	sealed = false
	mesh = _mesh
	lobby = _lobby
	connect_to_url(url)
	register_player(local_player_bytes)
	enable_polling()

func stop() -> void:
	multiplayer.multiplayer_peer = null
	rtc_mp.close()
	close()


func _create_peer(id: int) -> WebRTCPeerConnection:
	var peer: WebRTCPeerConnection = WebRTCPeerConnection.new()
	# Use a public STUN server for moderate NAT traversal.
	# Note that STUN cannot punch through strict NATs (such as most mobile connections),
	# in which case TURN is required. TURN generally does not have public servers available,
	# as it requires much greater resources to host (all traffic goes through
	# the TURN server, instead of only performing the initial connection).
	peer.initialize({
		"iceServers": [ { "urls": ["stun:stun.l.google.com:19302"] } ]
	})
	peer.session_description_created.connect(_offer_created.bind(id))
	peer.ice_candidate_created.connect(_new_ice_candidate.bind(id))
	rtc_mp.add_peer(peer, id)
	if id < rtc_mp.get_unique_id():  # So lobby creator never creates offers.
		peer.create_offer()
	return peer


func _new_ice_candidate(mid_name: String, index_name: int, sdp_name: String, id: int) -> void:
	send_candidate(id, mid_name, index_name, sdp_name)


func _offer_created(type: String, data: String, id: int) -> void:
	if not rtc_mp.has_peer(id):
		return
	print("created", type)
	rtc_mp.get_peer(id).connection.set_local_description(type, data)
	if type == "offer": send_offer(id, data)
	else: send_answer(id, data)


func _connected(id: int, use_mesh: bool) -> void:
	print("Connected %d, mesh: %s" % [id, use_mesh])
	if use_mesh:
		rtc_mp.create_mesh(id)
	elif id == 1:
		rtc_mp.create_server()
	else:
		rtc_mp.create_client(id)
	multiplayer.multiplayer_peer = rtc_mp
	player_id_received.emit(id)


func _lobby_joined(_lobby: String) -> void:
	lobby = _lobby


func _lobby_sealed() -> void:
	sealed = true


func _disconnected() -> void:
	print("Disconnected: %d: %s" % [code, reason])
	if not sealed:
		stop() # Unexpected disconnect


func _peer_connected(id: int) -> void:
	print("Peer connected: %d" % id)
	_create_peer(id)


func _peer_disconnected(id: int) -> void:
	if rtc_mp.has_peer(id):
		rtc_mp.remove_peer(id)


func _offer_received(id: int, offer: int) -> void:
	print("Got offer: %d" % id)
	if rtc_mp.has_peer(id):
		rtc_mp.get_peer(id).connection.set_remote_description("offer", offer)


func _answer_received(id: int, answer: int) -> void:
	print("Got answer: %d" % id)
	if rtc_mp.has_peer(id):
		rtc_mp.get_peer(id).connection.set_remote_description("answer", answer)


func _candidate_received(id: int, mid: String, index: int, sdp: String) -> void:
	if rtc_mp.has_peer(id):
		rtc_mp.get_peer(id).connection.add_ice_candidate(mid, index, sdp)
