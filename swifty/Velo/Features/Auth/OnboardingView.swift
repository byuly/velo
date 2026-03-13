import SwiftUI

struct OnboardingView: View {
    @Environment(AppState.self) private var appState
    @State private var displayName = ""
    @State private var isLoading = false

    private var isValid: Bool { !displayName.trimmingCharacters(in: .whitespaces).isEmpty }

    var body: some View {
        ZStack {
            Tokens.Colors.background.ignoresSafeArea()

            VStack(spacing: 0) {
                VStack(alignment: .leading, spacing: Tokens.Spacing.xl) {
                    Spacer().frame(height: Tokens.Spacing.xxxl)

                    VStack(alignment: .leading, spacing: Tokens.Spacing.sm) {
                        Text("What should we\ncall you?")
                            .font(.system(size: 32, weight: .bold))
                            .foregroundStyle(Tokens.Colors.foreground)
                            .lineSpacing(4)

                        Text("This is how you'll appear to friends in sessions.")
                            .font(Tokens.Typography.bodyBase)
                            .foregroundStyle(Tokens.Colors.mutedForeground)
                    }

                    VInput(
                        label: "Display Name",
                        placeholder: "e.g. Alex",
                        text: $displayName,
                        maxLength: 30,
                        submitLabel: .continue,
                        onSubmit: { if isValid { continue_() } }
                    )
                }
                .padding(.horizontal, Tokens.Spacing.xl)

                Spacer()

                VButton(
                    title: "Continue",
                    variant: .primary,
                    size: .lg,
                    isFullWidth: true,
                    isLoading: isLoading,
                    isDisabled: !isValid
                ) {
                    continue_()
                }
                .padding(.horizontal, Tokens.Spacing.xl)
                .padding(.bottom, Tokens.Spacing.xxxl)
            }
        }
    }

    private func continue_() {
        isLoading = true
        // Simulate async save
        Task {
            try? await Task.sleep(for: .milliseconds(400))
            await MainActor.run {
                appState.completeOnboarding(displayName: displayName.trimmingCharacters(in: .whitespaces))
                isLoading = false
            }
        }
    }
}

#Preview {
    OnboardingView()
        .environment(AppState())
}
