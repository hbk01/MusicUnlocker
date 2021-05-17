create database if not exists music_unlock;
use music_unlock;
drop table if exists logs;
create table if not exists logs
(
    id               integer primary key auto_increment,
    username         text not null,
    display_name     text not null,
    time             datetime not null,
    unlock_file_name text not null,
    unlock_file_size long not null
);