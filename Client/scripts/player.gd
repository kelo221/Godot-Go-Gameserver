extends CharacterBody3D

var local_player_data := PlayerProto.Player.new()

@onready var scoreboard_control: Control = $ScoreboardControl
@onready var player_health_label: Label3D = $PlayerHealthLabel
@onready var player_name_label: Label3D = $PlayerNameLabel
@onready var camera_3d: Camera3D = $Camera3D
@onready var mesh_instance_3d: MeshInstance3D = $MeshInstance3D
@onready var input_emulator_timer: Timer = $InputEmulatorTimer
const PROJECTILE = preload("res://scenes/projectile.tscn")
var emulate_input := false

var connection_tries := 0 
const SPEED = 5.0
const JUMP_VELOCITY = 4.5

const POLLING_TICK_BASE := 1

var is_puppet := false
var current_polling_tick := POLLING_TICK_BASE

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

func init_puppet() -> void:
	player_name_label.text = local_player_data.get_name()
	var new_color :=  StandardMaterial3D.new()
	new_color.albedo_color = Color(local_player_data.get_player_color())
	mesh_instance_3d.material_override = new_color

func player_disconnect(puppet : PlayerProto.Player) -> void:
	if local_player_data.get_id() == puppet.get_id():
		queue_free()

func init_player() -> void:
	local_player_data.set_player_color((Color(randf(), randf(), randf())).to_html())
	local_player_data.set_health(100)
	local_player_data.set_current_spell(0)
	local_player_data.set_casting(false)
	local_player_data.set_id(-1)
	local_player_data.set_rotation_x(0)
	local_player_data.set_rotation_y(0)
	local_player_data.set_player_state(0)
	local_player_data.set_name(Client.generate_name())

func _ready() -> void:
	
	set_physics_process(false)
	
	Client.connect("update_health", update_health)
	Client.connect("player_died", respawn)
	
	if not is_puppet:
		Client.connect("player_registed", _on_player_registed)
		Client.connect("player_new_position", set_spawn_position)
		init_player()
		
		Client.start()
		while Client.register(local_player_data.to_bytes()) == false:
			await get_tree().create_timer(0.01).timeout
			print("connecting... ", connection_tries, " tries.")
			connection_tries += 1
			
			if connection_tries > 10:
				printerr("Failed to connect!")
				break
		
		Client.send_empty(REQUEST_PLAYERS)
	else:
		init_puppet()
		Client.connect("player_disconnect", player_disconnect)
		Client.connect("puppet_fire_projectile", puppet_init_projectile)
		Client.connect("puppet_new_position", puppet_new_position)

func update_health (damage : PlayerProto.Player) -> void:
	if damage.get_id() == local_player_data.get_id():
		local_player_data = damage
		player_health_label.text = str(local_player_data.get_health()) + "/100"
		

func puppet_new_position(puppet : PlayerProto.Player):
	
	if local_player_data.get_id() == puppet.get_id():
		var new_position := puppet.get_pos()
		var new_pos_vec := Vector3(new_position[0].get_x(), new_position[0].get_y(), new_position[0].get_z())
		
		var current_rot_y := rotation.y
		var new_rot_y := puppet.get_rotation_y()
		
		# Normalize the new rotation angle to ensure it is between -π and π
		new_rot_y = wrapf(new_rot_y, -PI, PI)
		
		# Calculate the shortest path
		var delta_rot = wrapf(new_rot_y - current_rot_y, -PI, PI)
		var final_rot_y = current_rot_y + delta_rot
		
		
		position = position.slerp(new_pos_vec, .3)
		rotation.y = lerpf(rotation.y, final_rot_y, .3)


func set_spawn_position(_position : Vector3):
	global_position = _position

func _on_player_registed(player_data: PlayerProto.Player):
	local_player_data = player_data
	
	var new_color := StandardMaterial3D.new()
	new_color.albedo_color = Color.html(local_player_data.get_player_color())
	mesh_instance_3d.material_override = new_color
	
	set_physics_process(true)
	connected()

var gravity: float = ProjectSettings.get_setting("physics/3d/default_gravity")

func set_new_location(new_position : Vector3) -> void:
	self.global_position = new_position

func respawn(new_player_data : PlayerProto.Player) -> void:
	if new_player_data.get_id() == local_player_data.get_id():
		var new_position := new_player_data.get_pos()
		var new_pos_vec := Vector3(new_position[0].get_x(), new_position[0].get_y(), new_position[0].get_z())
		position = new_pos_vec
		player_health_label.text = "100/100"
		apply_new_position()  # Ensure new position is sent immediately

func get_id() -> int:
	return local_player_data.get_id()

func player_init_projectile() -> void:
	
	var damage_package := PlayerProto.Damage.new()
	damage_package.set_caster_id(local_player_data.get_id())
	
	Client.send(INIT_CAST, damage_package.to_bytes())
	var projectile_instance = PROJECTILE.instantiate()
	projectile_instance.position = global_transform.origin
	projectile_instance.rotation.y = rotation.y
	projectile_instance.set_meta("casted_by_puppet", false)
	projectile_instance.set_meta("Owner", local_player_data.get_id())
	get_tree().root.add_child(projectile_instance)

func puppet_init_projectile(caster_id : int) -> void:
	
	if caster_id == local_player_data.get_id():
		var projectile_instance = PROJECTILE.instantiate()
		projectile_instance.position = global_transform.origin
		projectile_instance.rotation.y = rotation.y
		projectile_instance.set_meta("casted_by_puppet", true)
		projectile_instance.set_meta("Owner", local_player_data.get_id())
		get_tree().root.add_child(projectile_instance)

func _physics_process(delta: float) -> void:
	
	if Input.is_action_just_pressed("f1"):
		input_emulator_timer.stop()
		emulate_input = !emulate_input
	
	if emulate_input:
		simulate_input()
	
	# Add the gravity.
	if not is_on_floor():
		velocity.y -= gravity * delta
		
	if Input.is_action_just_pressed("ui_accept") and is_on_floor():
		input_emulator_timer.stop()
		velocity.y = JUMP_VELOCITY
	
	if Input.is_action_just_pressed("fire"):
		player_init_projectile()
		
	if Input.is_action_just_pressed("tab"):
		scoreboard_control.visible = true
		
	if Input.is_action_just_released("tab"):
		scoreboard_control.visible = false
	
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
	
	if current_polling_tick == POLLING_TICK_BASE:
		Client.send(UPDATE_LOCATION, local_player_data.to_bytes())
	
	current_polling_tick += 1	
	
	if current_polling_tick > POLLING_TICK_BASE:
		current_polling_tick = 0

func simulate_input() -> void:
	Input.action_press("ui_accept")
	await get_tree().create_timer(0.5).timeout
	Input.action_release("ui_accept")

func _on_input_emulator_timeout() -> void:
	emulate_input = true

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
