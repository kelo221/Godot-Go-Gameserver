extends Node

var socket = WebSocketPeer.new()
var connection_state

var first_message := true

var remote_player_data := Protobuff.Players.new()
var local_player_bytes : PackedByteArray

var empty : PackedByteArray
var puppet_data_bytes : PackedByteArray
var puppet_data : Protobuff.Players

signal player_id_received(player_id: StringName)
signal player_new_position(new_position : Vector3)
signal puppet_new_position(new_position : Vector3, id : int)
signal new_puppet(new_position : Vector3, player_data : Protobuff.Player)

var player_id := 0

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
	player_id = int(body.get_string_from_utf8())
	socket.connect_to_url("ws://localhost:8080/websocket")
	set_physics_process(true)	
	player_id_received.emit(body.get_string_from_utf8())


func _physics_process(_delta: float) -> void:
	socket.poll()
	connection_state = socket.get_ready_state()
	if connection_state == WebSocketPeer.STATE_OPEN:
		
		
		if first_message:
			socket.send(empty)
		else:
			socket.send(local_player_bytes)
		
		
		
		while socket.get_available_packet_count():
			
			puppet_data_bytes = socket.get_packet()
			remote_player_data = Protobuff.Players.new()
			var state := remote_player_data.from_bytes(puppet_data_bytes)
			
			if state == Protobuff.PB_ERR.NO_ERRORS:
				
				if first_message:
					for player : Protobuff.Player in remote_player_data.get_player():
						if player.get_id() == player_id:
							var pos = player.get_pos()
							player_new_position.emit(Vector3(pos[0].get_x(), pos[0].get_y(), pos[0].get_z()))
						else:
							var pos = player.get_pos()
							new_puppet.emit(
								Vector3(pos[0].get_x(), pos[0].get_y(), pos[0].get_z()),
								player
								)
					first_message = false
				else:
					for player : Protobuff.Player in remote_player_data.get_player():
						if player.get_id() != player_id:
							var pos = player.get_pos()
							puppet_new_position.emit(Vector3(pos[0].get_x(), pos[0].get_y(), pos[0].get_z()), player.get_id())

			else:
				printerr(state)
				return
			
	elif connection_state == WebSocketPeer.STATE_CLOSING:
		printerr("Closing...")
	elif connection_state == WebSocketPeer.STATE_CLOSED:
		var code = socket.get_close_code()
		var reason = socket.get_close_reason()
		printerr("WebSocket closed with code: %d, reason %s. Clean: %s" % [code, reason, code != -1])
		set_process(false) # Stop processing.


#func _on_server_poll_timer_timeout() -> void:
	#if connection_state == WebSocketPeer.STATE_OPEN:
		#socket.send(local_player_bytes)


const NOUNS := ["Computer", "Mountain", "Ocean", "Book", "Music", "Television", "Apple", "City", "Car", "Space"];
const ADJECTIVES := ["Happy", "Sad", "Excited", "Angry", "Joyful", "Peaceful", "Grumpy", "Elated", "Nervous", "Relaxed"];

func generate_name() -> StringName:
	return ADJECTIVES.pick_random() + " " + NOUNS.pick_random()
