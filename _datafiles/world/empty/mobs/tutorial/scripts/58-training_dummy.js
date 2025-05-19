const teacherMobId = 57;

function onDie(mob, room, eventDetails) {

    room.SendText( mob.GetCharacterName(true) + " crumbles to dust." );

    teacherMob = room.GetMob(teacherMobId, true);

    teacherMob.Command('say You did it! Head <ansi fg="exit">west</ansi> to complete your training.');
}
