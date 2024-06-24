extends Node

enum  {
	REQUEST_PLAYERS,
	REGISTER,
	UPDATE_LOCATION,
	POLL_LOCATIONS,
	DAMAGE_PLAYER,
	INIT_CAST,
	RESPAWN_PLAYER,
	REQUEST_SCOREBOARD,
	PLAYER_DISCONNECT,
}


var peer := WebSocketPeer.new()
var local_player_id := 0


var scoreboard_data : ScoreboardProto.Scoreboard

var puppet_data_bytes : PackedByteArray
var puppet_data : PlayerProto.Players

signal player_registed(player: PlayerProto.Player)
signal player_new_position(new_position : Vector3)

signal puppet_fire_projectile(puppet: PlayerProto.Player)
signal puppet_new_position(puppet_id : int)
signal new_puppet(player_data : PlayerProto.Player)
signal player_disconnect(player_data : PlayerProto.Player)

signal update_health(player_damage : PlayerProto.Player)
signal update_scoreboard()
signal player_died(player: PlayerProto.Player)

var player_id := 0

const NOUNS := ["Computer", "Mountain", "Ocean", "Book", "Music", "Television", "Apple", "City", "Car", "Space"]
const ADJECTIVES := ["Happy", "Sad", "Excited", "Angry", "Joyful", "Peaceful", "Grumpy", "Elated", "Nervous", "Relaxed"]

func _ready() -> void:
	scoreboard_data = ScoreboardProto.Scoreboard.new()

func generate_name() -> StringName:
	return ADJECTIVES.pick_random() + " " + NOUNS.pick_random()	

@export var handshake_headers: PackedStringArray
@export var supported_protocols: PackedStringArray
var tls_options: TLSOptions = null

var socket := WebSocketPeer.new()
var last_state := WebSocketPeer.STATE_CLOSED

signal connected_to_server()
signal connection_closed()

func start(url = "ws://127.0.0.1:8080"):
	socket.supported_protocols = supported_protocols
	socket.handshake_headers = handshake_headers
	
	var err := socket.connect_to_url(url, tls_options)
	if err != OK:
		printerr(err)
		
	last_state = socket.get_ready_state()

func register(message: PackedByteArray) -> bool:
	
	while socket.get_ready_state() != WebSocketPeer.STATE_OPEN:
		return false
	
	message.insert(0 ,REGISTER)
	socket.send(message)
	return true

func send(message_type : int, message: PackedByteArray) -> void:
	message.insert(0 ,message_type)
	socket.send(message)
	
func send_empty(message_type : int) -> void:
	var empty_message : PackedByteArray
	empty_message.insert(0 ,message_type)
	socket.send(empty_message)

func get_message() -> void:
	if socket.get_available_packet_count() < 1:
		return
	
	var data : PackedByteArray = socket.get_packet()
	var message_type := data.decode_u8(0)
	var message_data := 	data.slice(1)
	
	match message_type:
		REQUEST_PLAYERS:
			register_puppets(message_data)
		REGISTER:
			register_local_player(message_data)
		UPDATE_LOCATION:
			update_puppet_positions(message_data)
		POLL_LOCATIONS:
			pass
		DAMAGE_PLAYER:
			update_player_health(message_data)
		INIT_CAST:
			set_puppet_cast(message_data)
		RESPAWN_PLAYER:
			handle_player_death(message_data)
		REQUEST_SCOREBOARD:
			handle_scoreboard(message_data)
		PLAYER_DISCONNECT:
			delete_puppet(message_data)
		_:
			printerr("Undefined message type: ", message_type)
			
func delete_puppet(message_data : PackedByteArray) -> void:
	var disconnected_player = PlayerProto.Player.new()
	var result = disconnected_player.from_bytes(message_data)
	
	if result == PlayerProto.PB_ERR.NO_ERRORS:
		player_disconnect.emit(disconnected_player)
	
	
func handle_scoreboard(message_data : PackedByteArray) -> void:
	var new_score = ScoreboardProto.Scoreboard.new()
	var result = new_score.from_bytes(message_data)
	
	if result == ScoreboardProto.PB_ERR.NO_ERRORS:
		scoreboard_data = new_score
		update_scoreboard.emit()

func handle_player_death(message_data : PackedByteArray) -> void:
	var dead_player = PlayerProto.Player.new()
	var result = dead_player.from_bytes(message_data)
	
	if result == PlayerProto.PB_ERR.NO_ERRORS:
		player_died.emit(dead_player)
	

func set_puppet_cast(message_data : PackedByteArray) -> void:
	var new_puppet_cast = PlayerProto.Damage.new()
	var result = new_puppet_cast.from_bytes(message_data)
	
	if result == PlayerProto.PB_ERR.NO_ERRORS:
		puppet_fire_projectile.emit(new_puppet_cast.get_caster_id())
	else :
		printerr("Unpacking failed.")


func update_player_health(message_data : PackedByteArray) -> void:
	var new_damage = PlayerProto.Player.new()
	var result = new_damage.from_bytes(message_data)
	
	if result == PlayerProto.PB_ERR.NO_ERRORS:
		update_health.emit(new_damage)
	else :
		printerr("Unpacking failed.")

func register_puppets(message_data : PackedByteArray) -> void:
	var new_puppet_data = PlayerProto.Players.new()
	var result = new_puppet_data.from_bytes(message_data)
	
	if result == PlayerProto.PB_ERR.NO_ERRORS:
		for puppet in new_puppet_data.get_player():
			if puppet.get_id() != local_player_id:
				new_puppet.emit(puppet)
	else :
		printerr("Unpacking failed.")

func register_local_player(message_data : PackedByteArray) -> void:
	var new_player_data = PlayerProto.Player.new()
	var result = new_player_data.from_bytes(message_data)
	if result == PlayerProto.PB_ERR.NO_ERRORS:
		if local_player_id == 0:
			local_player_id = new_player_data.get_id()
			player_registed.emit(new_player_data)
			var new_position := new_player_data.get_pos()
			player_new_position.emit(Vector3(new_position[0].get_x(), new_position[0].get_y(), new_position[0].get_z()))
	else:
		printerr("Unpacking failed.")


func update_puppet_positions(message_data : PackedByteArray) -> void:
	var new_location_data = PlayerProto.Players.new()
	var result = new_location_data.from_bytes(message_data)

	if result == PlayerProto.PB_ERR.NO_ERRORS:
		for puppet in new_location_data.get_player():
				puppet_new_position.emit(puppet)
	else :
		printerr("Unpacking failed.")

func close(code: int = 1000, reason: String = "") -> void:
	socket.close(code, reason)
	last_state = socket.get_ready_state()


func clear() -> void:
	socket = WebSocketPeer.new()
	last_state = socket.get_ready_state()


func get_socket() -> WebSocketPeer:
	return socket


func poll() -> void:
	if socket.get_ready_state() != socket.STATE_CLOSED:
		socket.poll()

	var state := socket.get_ready_state()

	if last_state != state:
		last_state = state
		if state == socket.STATE_OPEN:
			connected_to_server.emit()
		elif state == socket.STATE_CLOSED:
			connection_closed.emit()
	while socket.get_ready_state() == socket.STATE_OPEN and socket.get_available_packet_count():
		get_message()


func _physics_process(delta: float) -> void:
	poll()
