import SwiftUI

struct WelcomeView: View {
    @Environment(AppState.self) private var appState
    @State private var appeared = false

    var body: some View {
        ZStack {
            Tokens.Colors.background.ignoresSafeArea()

            VStack(spacing: 0) {
                Spacer()

                // Wordmark
                VStack(spacing: Tokens.Spacing.lg) {
                    Text("velo")
                        .font(.system(size: 56, weight: .bold, design: .default))
                        .foregroundStyle(Tokens.Colors.foreground)
                        .opacity(appeared ? 1 : 0)
                        .animation(.easeOut(duration: 0.5).delay(0.0), value: appeared)

                    VStack(spacing: Tokens.Spacing.sm) {
                        Text("Your day, shared.")
                            .font(.system(size: 20, weight: .semibold))
                            .foregroundStyle(Tokens.Colors.foreground)
                            .opacity(appeared ? 1 : 0)
                            .animation(.easeOut(duration: 0.5).delay(0.1), value: appeared)

                        Text("Record moments throughout the day.\nAt deadline, everyone's day plays side-by-side.")
                            .font(Tokens.Typography.bodyBase)
                            .foregroundStyle(Tokens.Colors.mutedForeground)
                            .multilineTextAlignment(.center)
                            .lineSpacing(4)
                            .opacity(appeared ? 1 : 0)
                            .animation(.easeOut(duration: 0.5).delay(0.2), value: appeared)
                    }
                }

                Spacer()

                // Sign in with Apple
                VStack(spacing: Tokens.Spacing.md) {
                    Button {
                        appState.login()
                    } label: {
                        HStack(spacing: Tokens.Spacing.sm) {
                            Image(systemName: "apple.logo")
                                .font(.system(size: 17, weight: .semibold))
                            Text("Sign in with Apple")
                                .font(.system(size: 17, weight: .semibold))
                        }
                        .foregroundStyle(.white)
                        .frame(maxWidth: .infinity)
                        .frame(height: 52)
                        .background(Color.black)
                        .clipShape(RoundedRectangle(cornerRadius: Tokens.Radius.base))
                    }
                    .buttonStyle(.plain)
                    .opacity(appeared ? 1 : 0)
                    .animation(.easeOut(duration: 0.5).delay(0.3), value: appeared)

                    Text("By continuing, you agree to our Terms and Privacy Policy.")
                        .font(Tokens.Typography.caption)
                        .foregroundStyle(Tokens.Colors.mutedForeground)
                        .multilineTextAlignment(.center)
                        .opacity(appeared ? 1 : 0)
                        .animation(.easeOut(duration: 0.5).delay(0.4), value: appeared)
                }
                .padding(.bottom, Tokens.Spacing.xxxl)
            }
            .padding(.horizontal, Tokens.Spacing.xl)
        }
        .onAppear { appeared = true }
    }
}

#Preview {
    WelcomeView()
        .environment(AppState())
}
