import SwiftUI

struct ReelPlayerView: View {
    @Environment(\.dismiss) private var dismiss
    let session: Session

    @State private var isPlaying = false
    @State private var progress: Double = 0
    @State private var playTimer: Timer? = nil
    @State private var savedToRoll = false

    // Simulated total duration ~45s
    private let totalDuration: Double = 45

    private var participants: [Participant] { session.participants }

    var body: some View {
        ZStack {
            Color.black.ignoresSafeArea()

            VStack(spacing: 0) {
                // Header
                HStack {
                    Button { dismiss() } label: {
                        Image(systemName: "xmark")
                            .font(.system(size: 14, weight: .semibold))
                            .foregroundStyle(.white)
                            .frame(width: 36, height: 36)
                            .background(Color.white.opacity(0.15))
                            .clipShape(Circle())
                    }
                    Spacer()
                    Text(session.name)
                        .font(.system(size: 15, weight: .semibold))
                        .foregroundStyle(.white)
                    Spacer()
                    Color.clear.frame(width: 36, height: 36)
                }
                .padding(.horizontal, Tokens.Spacing.base)
                .padding(.top, Tokens.Spacing.base)

                Spacer()

                // Expiry warning
                if session.isExpiringSoon {
                    HStack(spacing: Tokens.Spacing.sm) {
                        Image(systemName: "exclamationmark.triangle.fill")
                            .font(.system(size: 13))
                            .foregroundStyle(Tokens.Colors.warning)
                        Text("This reel expires in 15 days. Save it to keep it forever.")
                            .font(Tokens.Typography.bodySmall)
                            .foregroundStyle(Tokens.Colors.warning)
                    }
                    .padding(.horizontal, Tokens.Spacing.base)
                    .padding(.vertical, Tokens.Spacing.sm)
                    .background(Tokens.Colors.warningLight.opacity(0.15))
                    .overlay(
                        Rectangle()
                            .fill(Tokens.Colors.warning.opacity(0.4))
                            .frame(height: 1),
                        alignment: .bottom
                    )
                }

                // Split-screen panels
                reelPanels
                    .padding(.horizontal, Tokens.Spacing.base)

                Spacer()

                // Controls
                VStack(spacing: Tokens.Spacing.md) {
                    // Progress bar
                    VStack(spacing: Tokens.Spacing.xs) {
                        VProgressBar(
                            progress: progress,
                            height: 3,
                            foreground: .white,
                            background: Color.white.opacity(0.2)
                        )
                        .padding(.horizontal, Tokens.Spacing.base)

                        HStack {
                            Text(formatTime(progress * totalDuration))
                                .font(.system(size: 11, weight: .medium, design: .monospaced))
                                .foregroundStyle(Color.white.opacity(0.5))
                            Spacer()
                            Text(formatTime(totalDuration))
                                .font(.system(size: 11, weight: .medium, design: .monospaced))
                                .foregroundStyle(Color.white.opacity(0.5))
                        }
                        .padding(.horizontal, Tokens.Spacing.base)
                    }

                    // Playback button
                    Button {
                        togglePlayback()
                    } label: {
                        Image(systemName: isPlaying ? "pause.circle.fill" : "play.circle.fill")
                            .font(.system(size: 56))
                            .foregroundStyle(.white)
                    }
                    .buttonStyle(.plain)

                    // Save button
                    VButton(
                        title: savedToRoll ? "Saved!" : "Save to Camera Roll",
                        icon: savedToRoll ? "checkmark" : "square.and.arrow.down",
                        variant: .secondary,
                        size: .md,
                        isFullWidth: true,
                        isDisabled: savedToRoll
                    ) {
                        saveToRoll()
                    }
                    .padding(.horizontal, Tokens.Spacing.base)
                }
                .padding(.bottom, Tokens.Spacing.xxxl)
            }
        }
        .navigationBarHidden(true)
        .onDisappear { stopPlayback() }
    }

    // MARK: - Panels

    private var reelPanels: some View {
        let count = participants.count
        return Group {
            if count <= 2 {
                VStack(spacing: 2) {
                    ForEach(participants) { p in
                        panelView(for: p)
                    }
                }
            } else {
                VStack(spacing: 2) {
                    HStack(spacing: 2) {
                        ForEach(participants.prefix(2)) { p in
                            panelView(for: p)
                        }
                    }
                    if count > 2 {
                        HStack(spacing: 2) {
                            ForEach(participants.dropFirst(2)) { p in
                                panelView(for: p)
                            }
                            if count == 3 {
                                emptyPanel
                            }
                        }
                    }
                }
            }
        }
        .aspectRatio(9/16, contentMode: .fit)
        .clipShape(RoundedRectangle(cornerRadius: Tokens.Radius.md))
    }

    private func panelView(for participant: Participant) -> some View {
        ZStack(alignment: .bottomLeading) {
            Rectangle()
                .fill(panelColor(for: participant))

            if isPlaying {
                // Simulated "playing" shimmer
                Color.white.opacity(0.02)
            }

            Text(participant.displayName)
                .font(.system(size: 10, weight: .semibold))
                .foregroundStyle(.white.opacity(0.85))
                .padding(.horizontal, 6)
                .padding(.vertical, 3)
                .background(Color.black.opacity(0.4))
                .clipShape(RoundedRectangle(cornerRadius: 4))
                .padding(6)
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity)
    }

    private var emptyPanel: some View {
        Rectangle()
            .fill(Color(white: 0.08))
            .frame(maxWidth: .infinity, maxHeight: .infinity)
    }

    private func panelColor(for participant: Participant) -> Color {
        let colors: [Color] = [
            Color(hue: 150/360, saturation: 0.3, brightness: 0.25),
            Color(hue: 220/360, saturation: 0.3, brightness: 0.25),
            Color(hue: 280/360, saturation: 0.3, brightness: 0.25),
            Color(hue: 40/360, saturation: 0.3, brightness: 0.25),
        ]
        let idx = session.participants.firstIndex(where: { $0.id == participant.id }) ?? 0
        return colors[idx % colors.count]
    }

    // MARK: - Playback

    private func togglePlayback() {
        if isPlaying {
            stopPlayback()
        } else {
            startPlayback()
        }
    }

    private func startPlayback() {
        isPlaying = true
        playTimer = Timer.scheduledTimer(withTimeInterval: 0.05, repeats: true) { _ in
            progress = min(1.0, progress + 0.05 / totalDuration)
            if progress >= 1.0 {
                stopPlayback()
                progress = 0
            }
        }
    }

    private func stopPlayback() {
        isPlaying = false
        playTimer?.invalidate()
        playTimer = nil
    }

    private func saveToRoll() {
        // MVP: simulate save
        savedToRoll = true
    }

    private func formatTime(_ seconds: Double) -> String {
        let s = Int(seconds)
        return String(format: "%d:%02d", s / 60, s % 60)
    }
}

#Preview {
    NavigationStack {
        ReelPlayerView(session: MockData.sessions[1])
    }
    .environment(AppState())
}
