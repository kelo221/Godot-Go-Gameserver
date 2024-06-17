extends Node

enum Message {
	JOIN,
	ID,
	PEER_CONNECT,
	PEER_DISCONNECT,
	OFFER,
	ANSWER,
	CANDIDATE,
	SEAL,
	REGISTER,
	UPDATE_POSITION,
}

@export var autojoin := true
@export var lobby := ""  # Will create a new lobby if empty.
@export var mesh := true  # Will use the lobby host as relay otherwise.

var ws := WebSocketPeer.new()
var code := 1000
var reason := "Unknown"
var old_state := WebSocketPeer.STATE_CLOSED

signal lobby_joined(lobby: String)
signal connected(id: int, use_mesh: bool)
signal disconnected()
signal peer_connected(id: int)
signal peer_disconnected(id: int)
signal offer_received(id: int, offer: int)
signal answer_received(id: int, answer: int)
signal candidate_received(id: int, mid: String, index: int, sdp: String)
signal lobby_sealed()

func connect_to_url(url: String) -> void:
	close()
	code = 1000
	reason = "Unknown"
	ws.connect_to_url(url)
	
func _ready() -> void:
	set_physics_process(false)

func close() -> void:
	ws.close()

func enable_polling()->void:
	set_physics_process(true)

func _physics_process(delta: float) -> void:
	ws.poll()
	var state := ws.get_ready_state()
	if state != old_state and state == WebSocketPeer.STATE_OPEN and autojoin:
		join_lobby(lobby)
	while state == WebSocketPeer.STATE_OPEN and ws.get_available_packet_count():
		if not _parse_msg():
			print("Error parsing message from server.")
	if state != old_state and state == WebSocketPeer.STATE_CLOSED:
		code = ws.get_close_code()
		reason = ws.get_close_reason()
		disconnected.emit()
	old_state = state


func _parse_msg() -> bool:
	var parsed: Dictionary = JSON.parse_string(ws.get_packet().get_string_from_utf8())
	if typeof(parsed) != TYPE_DICTIONARY or not parsed.has("type") or not parsed.has("id") or \
		typeof(parsed.get("data")) != TYPE_STRING:
		return false

	var msg := parsed as Dictionary
	if not str(msg.type).is_valid_int() or not str(msg.id).is_valid_int():
		return false

	var type := str(msg.type).to_int()
	var src_id := str(msg.id).to_int()

	match type:
		Message.ID:
			connected.emit(src_id, msg.data == "true")
		Message.JOIN:
			lobby_joined.emit(msg.data)
		Message.PEER_CONNECT:
			peer_connected.emit(src_id)
		Message.SEAL:
			lobby_sealed.emit()
		Message.REGISTER:
			print("REGISTER message received: %s" % msg.data)
		Message.PEER_DISCONNECT:
			peer_disconnected.emit(src_id)
		Message.OFFER:
			offer_received.emit(src_id, msg.data)
		Message.ANSWER:
			answer_received.emit(src_id, msg.data)
		Message.CANDIDATE:
			var candidate: PackedStringArray = msg.data.split("\n", false)
			if candidate.size() != 3:
				return false
			if not candidate[1].is_valid_int():	
				return false
			candidate_received.emit(src_id, candidate[0], candidate[1].to_int(), candidate[2])
		Message.UPDATE_POSITION:  # Handle relay messages
			print("Relay message received: %s" % msg.data)
		_:
			return false

	return true  # Parsed.

func join_lobby(lobby: String) -> Error:
	return _send_msg(Message.JOIN, 0 if mesh else 1, lobby)

func seal_lobby() -> Error:
	return _send_msg(Message.SEAL, 0)

func send_candidate(id: int, mid: String, index: int, sdp: String) -> Error:
	return _send_msg(Message.CANDIDATE, id, "\n%s\n%d\n%s" % [mid, index, sdp])

func send_offer(id: int, offer: String) -> Error:
	return _send_msg(Message.OFFER, id, offer)

func register_player(data: PackedByteArray) -> Error:
	return _send_msg_bytes(Message.REGISTER, 0, data)

func send_answer(id: int, answer: String) -> Error:
	return _send_msg(Message.ANSWER, id, answer)

func update_position(data: String) -> Error:
	return _send_msg(Message.UPDATE_POSITION, 0, data)

func _send_msg(type: int, id: int, data: String = "") -> Error:
	return ws.send_text(JSON.stringify({
		"type": type,
		"id": id,
		"data": data,
	}))


func _send_msg_bytes(type: int, id: int, data: PackedByteArray) -> Error:
	return ws.send_text(JSON.stringify({
		"type": type,
		"id": id,
		"data": "data",
	}))
