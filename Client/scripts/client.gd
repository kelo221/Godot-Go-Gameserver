extends Node

var socket = WebSocketPeer.new()
var connection_state

var local_player_data : PackedByteArray

var puppet_data_raw : PackedByteArray
var puppet_data : Protobuff.Players

signal player_id_received(player_id: StringName)

func _ready() -> void:
	set_physics_process(false)

func register_player(player_name : StringName) -> void:
	var http_request = HTTPRequest.new()
	add_child(http_request)
	http_request.request_completed.connect(self._http_request_completed)
	var error = http_request.request("http://localhost:8080/register", [], HTTPClient.METHOD_POST, player_name)
	if error != OK:
		push_error("An error occurred in the HTTP request.")


# Called when the HTTP request is completed.
func _http_request_completed(_result, _response_code, _headers, body):
	socket.connect_to_url("ws://localhost:8080/websocket")
	set_physics_process(true)	
	player_id_received.emit(body.get_string_from_utf8())
	

	
func _physics_process(delta: float) -> void:
	socket.poll()
	connection_state = socket.get_ready_state()
	if connection_state == WebSocketPeer.STATE_OPEN:
		while socket.get_available_packet_count():
			
			puppet_data_raw = socket.get_packet()
			var a := Protobuff.Players.new()
			var state := a.from_bytes(puppet_data_raw)
			
			if state == Protobuff.PB_ERR.NO_ERRORS:
				print("OK")
			else:
				return
			
	elif connection_state == WebSocketPeer.STATE_CLOSING:
		pass
	elif connection_state == WebSocketPeer.STATE_CLOSED:
		var code = socket.get_close_code()
		var reason = socket.get_close_reason()
		print("WebSocket closed with code: %d, reason %s. Clean: %s" % [code, reason, code != -1])
		set_process(false) # Stop processing.


func _on_server_poll_timer_timeout() -> void:
	if connection_state == WebSocketPeer.STATE_OPEN:
		socket.send(local_player_data)


const NOUNS := ["Computer", "Mountain", "Ocean", "Book", "Music", "Television", "Apple", "City", "Car", "Space"];
const ADJECTIVES := ["Happy", "Sad", "Excited", "Angry", "Joyful", "Peaceful", "Grumpy", "Elated", "Nervous", "Relaxed"];

func generate_name() -> StringName:
	return NOUNS.pick_random() + " " + ADJECTIVES.pick_random()
