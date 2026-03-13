import SwiftUI

struct SettingsView: View {
    @Environment(AppState.self) private var appState
    @Environment(\.dismiss) private var dismiss

    @State private var displayName = ""
    @State private var showSignOutConfirm = false
    @State private var showDeleteConfirm = false
    @State private var deleteExpanded = false
    @State private var isSaving = false

    private var isNameChanged: Bool {
        displayName.trimmingCharacters(in: .whitespaces) != (appState.currentUser?.displayName ?? "")
        && !displayName.trimmingCharacters(in: .whitespaces).isEmpty
    }

    var body: some View {
        ScrollView {
            VStack(spacing: Tokens.Spacing.xl) {
                // Avatar + name
                VStack(spacing: Tokens.Spacing.md) {
                    VAvatar(
                        displayName: displayName.isEmpty ? "?" : displayName,
                        size: 72
                    )
                    VInput(
                        label: "Display Name",
                        placeholder: "Your name",
                        text: $displayName,
                        maxLength: 30
                    )
                    VButton(
                        title: "Save Changes",
                        variant: .primary,
                        size: .md,
                        isFullWidth: true,
                        isLoading: isSaving,
                        isDisabled: !isNameChanged
                    ) {
                        save()
                    }
                }
                .padding(.top, Tokens.Spacing.base)

                Divider()

                // Preferences section
                VStack(alignment: .leading, spacing: Tokens.Spacing.md) {
                    Text("Preferences")
                        .font(Tokens.Typography.label)
                        .foregroundStyle(Tokens.Colors.mutedForeground)
                        .textCase(.uppercase)
                        .kerning(0.5)

                    VCard {
                        VStack(spacing: 0) {
                            HStack {
                                VStack(alignment: .leading, spacing: 2) {
                                    Text("Active Session Intercept")
                                        .font(Tokens.Typography.bodyBase)
                                        .foregroundStyle(Tokens.Colors.foreground)
                                    Text("Open directly to your active session on launch")
                                        .font(Tokens.Typography.caption)
                                        .foregroundStyle(Tokens.Colors.mutedForeground)
                                }
                                Spacer()
                                Toggle("", isOn: Bindable(appState).activeSessionIntercept)
                                    .labelsHidden()
                                    .tint(Tokens.Colors.primary)
                                    .disabled(appState.activeSession == nil)
                            }
                            .padding(.vertical, Tokens.Spacing.md)
                        }
                    }
                }

                // About section
                VStack(alignment: .leading, spacing: Tokens.Spacing.md) {
                    Text("About")
                        .font(Tokens.Typography.label)
                        .foregroundStyle(Tokens.Colors.mutedForeground)
                        .textCase(.uppercase)
                        .kerning(0.5)

                    VCard {
                        VStack(spacing: 0) {
                            infoRow(label: "Version", value: "1.0.0 (1)")
                            Divider().padding(.horizontal, -Tokens.Spacing.base)
                            infoRow(label: "Reel Retention", value: "90 days")
                            Divider().padding(.horizontal, -Tokens.Spacing.base)
                            infoRow(label: "Max Participants", value: "4 per session")
                        }
                    }
                }

                // Account section
                VStack(alignment: .leading, spacing: Tokens.Spacing.md) {
                    Text("Account")
                        .font(Tokens.Typography.label)
                        .foregroundStyle(Tokens.Colors.mutedForeground)
                        .textCase(.uppercase)
                        .kerning(0.5)

                    VButton(
                        title: "Sign Out",
                        icon: "arrow.right.square",
                        variant: .secondary,
                        size: .md,
                        isFullWidth: true
                    ) {
                        showSignOutConfirm = true
                    }

                    // Delete account — expandable
                    VStack(spacing: Tokens.Spacing.sm) {
                        Button {
                            withAnimation(.easeInOut(duration: 0.2)) {
                                deleteExpanded.toggle()
                            }
                        } label: {
                            HStack {
                                Image(systemName: "trash")
                                    .font(.system(size: 14, weight: .medium))
                                Text("Delete Account")
                                    .font(.system(size: 15, weight: .semibold))
                                Spacer()
                                Image(systemName: deleteExpanded ? "chevron.up" : "chevron.down")
                                    .font(.system(size: 12))
                            }
                            .foregroundStyle(Tokens.Colors.destructive)
                            .frame(maxWidth: .infinity)
                            .frame(height: 44)
                            .padding(.horizontal, Tokens.Spacing.base)
                            .background(Tokens.Colors.destructiveLight)
                            .clipShape(RoundedRectangle(cornerRadius: Tokens.Radius.base))
                        }
                        .buttonStyle(.plain)

                        if deleteExpanded {
                            VStack(alignment: .leading, spacing: Tokens.Spacing.md) {
                                Text("This will permanently delete your account, all your clips, and remove you from any active sessions. This cannot be undone.")
                                    .font(Tokens.Typography.bodySmall)
                                    .foregroundStyle(Tokens.Colors.mutedForeground)

                                VButton(
                                    title: "Confirm Delete",
                                    variant: .destructive,
                                    size: .md,
                                    isFullWidth: true
                                ) {
                                    showDeleteConfirm = true
                                }
                            }
                            .padding(Tokens.Spacing.md)
                            .background(Tokens.Colors.destructiveLight.opacity(0.5))
                            .clipShape(RoundedRectangle(cornerRadius: Tokens.Radius.md))
                        }
                    }
                }

                Spacer().frame(height: Tokens.Spacing.xxxl)
            }
            .padding(.horizontal, Tokens.Spacing.base)
        }
        .background(Tokens.Colors.backgroundSubtle.ignoresSafeArea())
        .navigationTitle("Settings")
        .navigationBarTitleDisplayMode(.inline)
        .onAppear {
            displayName = appState.currentUser?.displayName ?? ""
        }
        .confirmationDialog("Sign Out", isPresented: $showSignOutConfirm, titleVisibility: .visible) {
            Button("Sign Out", role: .destructive) {
                appState.signOut()
            }
            Button("Cancel", role: .cancel) {}
        } message: {
            Text("You'll need to sign in again to use Velo.")
        }
        .confirmationDialog("Delete Account", isPresented: $showDeleteConfirm, titleVisibility: .visible) {
            Button("Delete Account", role: .destructive) {
                appState.signOut()
            }
            Button("Cancel", role: .cancel) {}
        } message: {
            Text("This action cannot be undone.")
        }
    }

    private func infoRow(label: String, value: String) -> some View {
        HStack {
            Text(label)
                .font(Tokens.Typography.bodyBase)
                .foregroundStyle(Tokens.Colors.foreground)
            Spacer()
            Text(value)
                .font(Tokens.Typography.bodyBase)
                .foregroundStyle(Tokens.Colors.mutedForeground)
        }
        .padding(.vertical, Tokens.Spacing.md)
    }

    private func save() {
        isSaving = true
        Task {
            try? await Task.sleep(for: .milliseconds(500))
            await MainActor.run {
                appState.updateUserDisplayName(displayName.trimmingCharacters(in: .whitespaces))
                isSaving = false
            }
        }
    }
}

#Preview {
    let appState = AppState()
    appState.currentUser = MockData.currentUser

    return NavigationStack {
        SettingsView()
    }
    .environment(appState)
}
