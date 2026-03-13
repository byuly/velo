import SwiftUI

struct HomeView: View {
    @Environment(AppState.self) private var appState
    @State private var selectedDate = Date()

    private var sessionDates: Set<Date> {
        appState.datesWithSessions(in: selectedDate)
    }

    private var selectedDateSessions: [Session] {
        appState.sessionsForDate(selectedDate)
    }

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: Tokens.Spacing.xl) {
                // Header
                HStack(alignment: .top) {
                    VStack(alignment: .leading, spacing: 4) {
                        Text("Welcome back,")
                            .font(Tokens.Typography.bodyBase)
                            .foregroundStyle(Tokens.Colors.mutedForeground)
                        Text(appState.currentUser?.displayName ?? "")
                            .font(.system(size: 24, weight: .bold))
                            .foregroundStyle(Tokens.Colors.foreground)
                    }
                    Spacer()
                    NavigationLink(value: NavDestination.settings) {
                        Image(systemName: "gearshape")
                            .font(.system(size: 16, weight: .medium))
                            .foregroundStyle(Tokens.Colors.mutedForeground)
                            .frame(width: 40, height: 40)
                            .background(Tokens.Colors.muted)
                            .clipShape(Circle())
                    }
                }
                .padding(.horizontal, Tokens.Spacing.base)
                .padding(.top, Tokens.Spacing.sm)

                // Active session banner
                if let active = appState.activeSession {
                    NavigationLink(value: NavDestination.session(active)) {
                        HStack {
                            VStack(alignment: .leading, spacing: 4) {
                                HStack(spacing: 6) {
                                    Circle()
                                        .fill(Color.white.opacity(0.7))
                                        .frame(width: 6, height: 6)
                                    Text("Active Session")
                                        .font(Tokens.Typography.caption)
                                        .fontWeight(.semibold)
                                        .foregroundStyle(Color.white.opacity(0.85))
                                }
                                Text(active.name)
                                    .font(Tokens.Typography.h3)
                                    .foregroundStyle(Color.white)
                                Text("Ends in \(active.timeUntilDeadline)")
                                    .font(Tokens.Typography.bodySmall)
                                    .foregroundStyle(Color.white.opacity(0.8))
                            }
                            Spacer()
                            Image(systemName: "arrow.right.circle.fill")
                                .font(.system(size: 24))
                                .foregroundStyle(Color.white)
                        }
                        .padding(Tokens.Spacing.base)
                        .background(Tokens.Colors.primary)
                        .clipShape(RoundedRectangle(cornerRadius: Tokens.Radius.base))
                    }
                    .buttonStyle(.plain)
                    .padding(.horizontal, Tokens.Spacing.base)
                }

                // Calendar
                VCard(padding: Tokens.Spacing.base) {
                    CalendarGridView(
                        selectedDate: $selectedDate,
                        sessionDates: sessionDates
                    ) { date in
                        selectedDate = date
                    }
                }
                .padding(.horizontal, Tokens.Spacing.base)

                // Sessions for selected date
                VStack(alignment: .leading, spacing: Tokens.Spacing.md) {
                    HStack {
                        Text(sectionTitle)
                            .font(.system(size: 14, weight: .semibold))
                            .foregroundStyle(Tokens.Colors.mutedForeground)
                            .textCase(.uppercase)
                            .kerning(0.5)
                        Spacer()
                    }
                    .padding(.horizontal, Tokens.Spacing.base)

                    if selectedDateSessions.isEmpty {
                        VStack(spacing: Tokens.Spacing.sm) {
                            Image(systemName: "calendar.badge.plus")
                                .font(.system(size: 32))
                                .foregroundStyle(Tokens.Colors.border)
                            Text("No sessions on this day")
                                .font(Tokens.Typography.bodyBase)
                                .foregroundStyle(Tokens.Colors.mutedForeground)
                            Text("Tap + to start a new session")
                                .font(Tokens.Typography.bodySmall)
                                .foregroundStyle(Tokens.Colors.mutedForeground.opacity(0.7))
                        }
                        .frame(maxWidth: .infinity)
                        .padding(.vertical, Tokens.Spacing.xxxl)
                    } else {
                        ForEach(selectedDateSessions) { session in
                            NavigationLink(value: session.status == .complete
                                ? NavDestination.reelPlayer(session)
                                : NavDestination.session(session)
                            ) {
                                SessionCard(session: session)
                            }
                            .buttonStyle(.plain)
                            .padding(.horizontal, Tokens.Spacing.base)
                            .disabled(session.status == .generating || session.status == .failed || session.status == .cancelled)
                        }
                    }
                }

                Spacer().frame(height: Tokens.Spacing.xxxl)
            }
        }
        .navigationBarHidden(true)
        .background(Tokens.Colors.backgroundSubtle.ignoresSafeArea())
        .overlay(alignment: .bottomTrailing) {
            // FAB
            NavigationLink(value: NavDestination.createSession) {
                Image(systemName: "plus")
                    .font(.system(size: 22, weight: .semibold))
                    .foregroundStyle(.white)
                    .frame(width: 56, height: 56)
                    .background(Tokens.Colors.primary)
                    .clipShape(Circle())
                    .shadow(
                        color: Tokens.Colors.primary.opacity(0.3),
                        radius: 12, x: 0, y: 6
                    )
            }
            .padding(.trailing, Tokens.Spacing.xl)
            .padding(.bottom, Tokens.Spacing.xxxl)
        }
        .navigationDestination(for: NavDestination.self) { destination in
            switch destination {
            case .settings:
                SettingsView()
            case .createSession:
                CreateSessionView()
            case .session(let session):
                SessionView(session: session)
            case .reelPlayer(let session):
                ReelPlayerView(session: session)
            case .camera(let slot, let session):
                CameraView(slot: slot, session: session)
            }
        }
    }

    private var sectionTitle: String {
        if Calendar.current.isDateInToday(selectedDate) { return "Today" }
        if Calendar.current.isDateInYesterday(selectedDate) { return "Yesterday" }
        let formatter = DateFormatter()
        formatter.dateFormat = "EEEE, MMM d"
        return formatter.string(from: selectedDate)
    }
}

// MARK: - Navigation

enum NavDestination: Hashable {
    case settings
    case createSession
    case session(Session)
    case reelPlayer(Session)
    case camera(SessionSlot, Session)
}

#Preview {
    let appState = AppState()
    appState.isAuthenticated = true
    appState.hasCompletedOnboarding = true
    appState.currentUser = MockData.currentUser

    return NavigationStack {
        HomeView()
    }
    .environment(appState)
}
