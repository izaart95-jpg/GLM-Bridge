# AI Coding Assistant Usage Guidelines

## Overview
This document outlines best practices and guidelines for utilizing various AI coding assistants in our development workflow. Each tool has specific strengths and optimal use cases that should be considered when selecting the appropriate assistant for different tasks.

## Tool Analysis & Recommendations

### 1. Claude Code (with CCR)
**Status:** Limited Usage Recommended
**Primary Use Case:** Technical questions and architectural guidance
**Best For:**
- Architecture discussions and high-level design decisions
- Technical problem-solving and conceptual guidance
- Reviewing code patterns and suggesting improvements

**Limitations:**
- Not recommended for direct code generation or implementation
- Should be used primarily for asking questions rather than writing code

### 2. Cline/Blackbox AI
**Status:** Conditional Recommendation
**Strengths:**
- Effective for certain coding patterns and implementations
- Good performance with standard development tasks

**Considerations:**
- May produce errors when working with complex or specialized code patterns
- Best results achieved with clear, well-defined requirements
- Testing and validation recommended for generated code

### 3. Cline/Kilo
**Status:** Advanced Capabilities Available
**Special Feature:** Browser action capability (`<browser_action>`)
**Use Cases:**
- Tasks requiring browser interaction or automation
- Web scraping and data extraction workflows
- UI testing and interaction scenarios

### 4. OpenCode
**Status:** Generally Effective
**Performance:** Works well for most development tasks
**Position:** Good overall performer but not necessarily the optimal choice for all scenarios
**Best For:**
- General programming tasks
- Language-agnostic code generation
- When other specialized tools are unavailable

## Best Practices

### Tool Selection Criteria
1. **Task Nature:** Match tool capabilities to task requirements
2. **Complexity Level:** Consider tool performance with complex code patterns
3. **Validation:** Always review and test AI-generated code
4. **Special Features:** Leverage unique capabilities (e.g., browser actions)

### Workflow Recommendations
1. Use Claude Code for architectural decisions and high-level guidance
2. Employ Cline/Blackbox for straightforward implementation tasks
3. Utilize Cline/Kilo for browser-related automation needs
4. Consider OpenCode as a reliable general-purpose alternative
5. Validate all AI-generated code through testing and peer review

### Quality Assurance
- Always review generated code for security vulnerabilities
- Test functionality thoroughly before integration
- Consider performance implications of AI-suggested implementations
- Maintain coding standards and style consistency

## Implementation Notes
- Document any significant AI-assisted code sections
- Track performance metrics for different tools on various task types
- Share learnings about effective prompting strategies
- Regularly update this document with new insights and tool evaluations

## Version History
- **2026-01-24:** Initial professional version created from informal notes
- Maintain updates as tool capabilities evolve and new insights emerge