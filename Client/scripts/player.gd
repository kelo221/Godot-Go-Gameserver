extends CharacterBody3D

var local_player_data := Protobuff.Player.new()
var multiplayer_id := -1
@onready var player_name_label: Label3D = $PlayerNameLabel

@onready var camera_3d: Camera3D = $Camera3D

const SPEED = 5.0
const JUMP_VELOCITY = 4.5

var name_received := false

var polling_tick := false

# Puppet data
var sync_pos_x : float
var sync_pos_y : float
var sync_pos_z : float
var sync_rot_y : float

func apply_new_position() ->void:
	local_player_data.clear_pos()
	var _pos
	_pos = local_player_data.add_pos()
	_pos.set_x(global_position.x)
	_pos.set_y(global_position.y)
	_pos.set_z(global_position.z)
	
func apply_new_rotation() ->void:
	local_player_data.set_rotation_x(rotation.x)
	local_player_data.set_rotation_y(rotation.y)

func connected() -> void:
	player_name_label.text = local_player_data.get_name()
	if Client.player_id == local_player_data.get_id():
		camera_3d.current = true
	
func _ready() -> void:
	set_physics_process(false)
	local_player_data.set_health(100)
	local_player_data.set_current_spell(0)
	local_player_data.set_casting(false)
	local_player_data.set_id(-1)
	local_player_data.set_rotation_x(0)
	local_player_data.set_rotation_y(0)
	Client.connect("player_id_received", _on_player_id_received)
	Client.connect("puppet_new_position", puppet_new_position)
	Client.connect("player_new_position", set_spawn_position)
	local_player_data.set_name(Client.generate_name())
	Client.local_player_bytes = local_player_data.to_bytes()
	Client.start()
	

func puppet_new_position(_position : Vector3, id :int):
	if local_player_data.get_id() == id:
		global_position = _position

func set_spawn_position(_position : Vector3):
	global_position = _position


func _on_player_id_received(player_id: int):
	local_player_data.set_id(player_id)
	set_physics_process(true)
	connected()

var gravity: float = ProjectSettings.get_setting("physics/3d/default_gravity")

func set_new_location(new_position : Vector3) -> void:
	self.global_position = new_position

func _physics_process(delta: float) -> void:
	
	if local_player_data.get_id() == 0:
		return
	
	# Add the gravity.
	if not is_on_floor():
		velocity.y -= gravity * delta
		
	if Input.is_action_just_pressed("ui_accept") and is_on_floor():
		velocity.y = JUMP_VELOCITY
		
	
	if Input.is_action_pressed("rotate_right") and !Input.is_action_pressed("rotate_left") :
		rotation.y -= .05
		apply_new_rotation()
		
	elif Input.is_action_pressed("rotate_left") and !Input.is_action_pressed("rotate_right"):
		rotation.y += .05
		apply_new_rotation()

	var input_dir := Input.get_vector("left", "right", "forward", "backward")
	var direction := (transform.basis * Vector3(input_dir.x, 0, input_dir.y)).normalized()
	if direction:
		velocity.x = direction.x * SPEED
		velocity.z = direction.z * SPEED
	else:
		velocity.x = move_toward(velocity.x, 0, SPEED)
		velocity.z = move_toward(velocity.z, 0, SPEED)
	
	if velocity != Vector3():
		apply_new_position()
	
	move_and_slide()
	
	
	if polling_tick == true:
		Client.local_player_bytes = local_player_data.to_bytes()
	
	polling_tick = !polling_tick
