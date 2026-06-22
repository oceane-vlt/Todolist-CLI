package main

import (
	"fmt"

	"github.com/manifoldco/promptui"
	"github.com/oceane-vlt/todolist/libs/clientauth"
	"github.com/oceane-vlt/todolist/libs/ui"
	"github.com/spf13/cobra"
)

// loginCmd authenticates an existing user. The password is read from a masked
// interactive prompt, never a flag, so it never leaks into the shell history,
// the process list or logs (see TÂCHE 5 clarification in the design notes).
var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Log in to your remote todo account",
	Long: `Log in with your email and password.
The password is requested via a masked interactive prompt (not a flag).
On success the access/refresh tokens are stored in ~/.config/todolist/credentials.json (0600).`,
	Run: func(cmd *cobra.Command, args []string) {
		email, _ := cmd.Flags().GetString("email")
		runAuth(cmd, email, "Log in", false)
	},
}

// signupCmd creates a new account. Same UX as login in the local/home dev mode;
// against Supabase Auth it would trigger account creation (and possibly email
// verification) instead.
var signupCmd = &cobra.Command{
	Use:   "signup",
	Short: "Create a new remote todo account",
	Long: `Create an account with your email and password.
The password is requested via a masked interactive prompt (not a flag).`,
	Run: func(cmd *cobra.Command, args []string) {
		email, _ := cmd.Flags().GetString("email")
		runAuth(cmd, email, "Sign up", true)
	},
}

// logoutCmd removes the local credentials file.
var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Log out (delete locally stored credentials)",
	Run: func(cmd *cobra.Command, args []string) {
		path, err := clientauth.CredentialsPath()
		if err != nil {
			ui.Error(err.Error())
			return
		}
		if err := clientauth.Delete(path); err != nil {
			ui.Error(err.Error())
			return
		}
		ui.Success("Logged out.")
	},
}

// runAuth performs the shared signup/login flow: ensure an email, prompt for the
// password, obtain credentials via the configured authenticator, and persist
// them with 0600 permissions. When isSignup is true and the authenticator
// supports a distinct account-creation endpoint (Supabase), SignUp is used;
// otherwise login (Authenticate) is used — in the local/home dev mode signup and
// login are the same operation.
func runAuth(_ *cobra.Command, email, action string, isSignup bool) {
	if email == "" {
		prompt := promptui.Prompt{Label: "Email"}
		entered, err := prompt.Run()
		if err != nil {
			ui.Error("Email is required.")
			return
		}
		email = entered
	}

	pwPrompt := promptui.Prompt{Label: "Password", Mask: '*'}
	password, err := pwPrompt.Run()
	if err != nil {
		ui.Error("Password is required.")
		return
	}

	authenticator, err := clientauth.NewAuthenticatorFromEnv()
	if err != nil {
		ui.Error("No authentication mode configured.")
		ui.Info("For Supabase Auth, set " +
			ui.Command(clientauth.EnvSupabaseURL+"=<https://ref.supabase.co>") + " and " +
			ui.Command(clientauth.EnvSupabaseAnonKey+"=<anon-key>") + ".")
		ui.Info("For local/dev use, set " + ui.Command(clientauth.EnvJWTSigningKey+"=<shared-secret>") +
			" (same value as the server). See docs/deployment.md and docs/target-architecture.md §5.")
		return
	}

	creds, err := obtainCredentials(authenticator, email, password, isSignup)
	if err != nil {
		ui.Error(err.Error())
		return
	}

	path, err := clientauth.CredentialsPath()
	if err != nil {
		ui.Error(err.Error())
		return
	}
	if err := clientauth.Save(path, creds); err != nil {
		ui.Error(err.Error())
		return
	}

	ui.Success(fmt.Sprintf("%s successful as %s.", action, creds.Email))
	ui.Info("Credentials stored at " + path + " (0600).")
}

// obtainCredentials calls the right operation on the authenticator. For signup
// it uses the dedicated SignUp endpoint when available (SignUpAuthenticator,
// i.e. Supabase); otherwise — and for login — it falls back to Authenticate (the
// local/home dev mode where signup and login are equivalent).
func obtainCredentials(authenticator clientauth.Authenticator, email, password string, isSignup bool) (*clientauth.Credentials, error) {
	if isSignup {
		if su, ok := authenticator.(clientauth.SignUpAuthenticator); ok {
			return su.SignUp(email, password, serverEndpoint())
		}
	}
	return authenticator.Authenticate(email, password, serverEndpoint())
}

func init() {
	loginCmd.Flags().StringP("email", "e", "", "Account email (password is prompted)")
	signupCmd.Flags().StringP("email", "e", "", "Account email (password is prompted)")
	rootCmd.AddCommand(loginCmd)
	rootCmd.AddCommand(signupCmd)
	rootCmd.AddCommand(logoutCmd)
}
