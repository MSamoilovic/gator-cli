package main

import (
	"context"
	"fmt"
	"time"

	"gator-ci/internal/database"

	"github.com/google/uuid"
)

func handlerLogin(s *state, cmd command) error {
	if cmd.Name != "login" {
		return fmt.Errorf("Command is not login")
	}

	if len(cmd.Args) != 1 {
		return fmt.Errorf("username required to log in")
	}

	dbUser, err := s.Db.GetUser(context.Background(), cmd.Args[0])
	if err != nil {
		return fmt.Errorf("user doesn't exist: %v", err)
	}

	err = s.Cfg.SetUsername(dbUser.Name)
	if err != nil {
		return fmt.Errorf("can't log in: %v", err)
	}

	fmt.Printf("User %s logged on\n", s.Cfg.CurrentUserName)

	return nil
}

func handlerUsers(s *state, _ command) error {
	users, err := s.Db.GetUsers(context.Background())
	if err != nil {
		return fmt.Errorf("error fetching users: %v", err)
	}

	for _, u := range users {
		if u.Name == s.Cfg.CurrentUserName {
			fmt.Printf("* %s (current)\n", u.Name)
		} else {
			fmt.Printf("* %s\n", u.Name)
		}
	}
	return nil
}

func handlerReset(s *state, cmd command) error {
	if err := s.Db.DeleteAllUsers(context.Background()); err != nil {
		return fmt.Errorf("error resetting database: %v", err)
	}
	fmt.Println("Database reset successfully")
	return nil
}

func handlerRegister(s *state, cmd command) error {
	if cmd.Name != "register" {
		return fmt.Errorf("program error, not register")
	}
	
	if len(cmd.Args) != 1 {
		return fmt.Errorf("username required to register")
	}

	params := database.CreateUserParams{
		ID:        uuid.New(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Name:      cmd.Args[0],
	}
	dbUser, err := s.Db.CreateUser(context.Background(), params)
	if err != nil {
		return fmt.Errorf("error creating user: %v", err)
	}

	if err := s.Cfg.SetUsername(dbUser.Name); err != nil {
		return fmt.Errorf("error setting username: %v", err)
	}
	fmt.Println("User created successfully")
	fmt.Printf("User %s created", s.Cfg.CurrentUserName)

	return nil
}