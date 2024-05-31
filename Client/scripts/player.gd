extends CharacterBody3D

var local_player_data := Protobuff.Player.new()
var multiplayer_id := -1

const SPEED = 5.0
const JUMP_VELOCITY = 4.5

var name_received := false

# Puppet data
var sync_pos_x : float
var sync_pos_y : float
var sync_pos_z : float
var sync_rot_y : float

func apply_new_position() ->void:
	var _pos
	_pos = local_player_data.add_pos()
	_pos.set_x(snapped(global_position.x, 0.01))
	_pos.set_y(snapped(global_position.y, 0.01))
	_pos.set_z(snapped(global_position.z, 0.01))
	
func apply_new_rotation() ->void:
	var _rot
	_rot = local_player_data.add_rot()
	_rot.set_x(rotation.x)
	_rot.set_y(rotation.y)
	_rot.set_z(rotation.z)

func _ready() -> void:
	local_player_data.set_health(100)
	Client.connect("player_id_received", _on_player_id_received)
	local_player_data.set_name(Client.generate_name())
	Client.register_player(local_player_data.get_name())

func _on_player_id_received(player_id: String):
	local_player_data.set_id(int(player_id))
	print("signal set")
	name_received = true


# Get the gravity from the project settings to be synced with RigidBody nodes.
var gravity: float = ProjectSettings.get_setting("physics/3d/default_gravity")

func set_new_location(new_position : Vector3) -> void:
	self.global_position = new_position

func _physics_process(delta: float) -> void:
	
	if !name_received:
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
	
	if name_received:
		print("packed")
		Client.local_player_data = local_player_data.to_bytes()
